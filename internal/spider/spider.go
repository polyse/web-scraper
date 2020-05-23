package spider

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"github.com/gocolly/colly/v2"
	sdk "github.com/polyse/database-sdk"
	zl "github.com/rs/zerolog/log"
	"go.uber.org/ratelimit"
	"go.zoe.im/surferua"

	"github.com/polyse/web-scraper/internal/extractor"
	"github.com/polyse/web-scraper/internal/rabbitmq"
)

type Spider struct {
	Queue              *rabbitmq.Queue
	RateLimit          ratelimit.Limiter
	Delay, RandomDelay time.Duration
	userAgentMutex     *sync.RWMutex
	dataCh             chan sdk.RawData
}

func NewSpider(queue *rabbitmq.Queue, limit int, delay, randomDelay time.Duration) (*Spider, error) {
	s := &Spider{
		Queue:          queue,
		RateLimit:      ratelimit.New(limit),
		Delay:          delay,
		RandomDelay:    randomDelay,
		userAgentMutex: &sync.RWMutex{},
		dataCh:         make(chan sdk.RawData),
	}
	go s.Listener()
	return s, nil
}

func (s *Spider) Scrap(ctx context.Context, startUrl *url.URL) error {
	subCtx, cancel := context.WithCancel(ctx)
	co, err := s.initScrapper(subCtx, startUrl)
	if err != nil {
		return err
	}
	if err := co.Visit(startUrl.String()); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			default:
			}
			<-time.After(5 * time.Second)
			s.userAgentMutex.Lock()
			co.UserAgent = surferua.New().String()
			s.userAgentMutex.Unlock()
		}
	}()
	go func() {
		co.Wait()
		cancel()
		zl.Debug().Msgf("%s", co.String())
		zl.Debug().Msgf("Finish %v", startUrl)
	}()
	return nil
}

func (s *Spider) initScrapper(ctx context.Context, u *url.URL) (*colly.Collector, error) {
	co := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent(surferua.New().String()),
		colly.AllowedDomains(u.Host),
	)
	err := co.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: runtime.NumCPU(),
		Delay:       s.Delay,
		RandomDelay: s.RandomDelay,
	})
	if err != nil {
		return nil, fmt.Errorf("can not create limit: %w", err)
	}
	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		link := e.Attr("href")
		fullLink := e.Request.AbsoluteURL(link)
		zl.Debug().Msgf("Find URL : %v", fullLink)
		s.RateLimit.Take()
		s.userAgentMutex.RLock()
		err := e.Request.Visit(fullLink)
		s.userAgentMutex.RUnlock()
		if err == colly.ErrAlreadyVisited || err == colly.ErrForbiddenDomain {
			return
		}
		if err != nil {
			zl.Warn().Err(err).Msgf("Can't visit page : %v", fullLink)
		}
	})
	co.OnResponse(func(r *colly.Response) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload := string(r.Body[:])
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload))
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}

		title := doc.Find("Title").Text()

		actual, err := extractor.ExtractContentFromHTML(payload)
		if err != nil {
			zl.Debug().Err(err).Msgf("Can't parse")
			return
		}
		content := extractor.Clean(actual)

		t := s.searchDate(r, doc)

		s.dataCh <- sdk.RawData{
			Source: sdk.Source{
				Date:  t,
				Title: title,
			},
			Url:  filepath.Join(r.Request.URL.Host, r.Request.URL.Path),
			Data: content,
		}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msgf("Can't connect to URL %s", filepath.Join(r.Request.URL.Host, r.Request.URL.Path))
	})
	return co, nil
}

func (s *Spider) searchDate(r *colly.Response, doc *goquery.Document) time.Time {
	yearBefore := time.Now().Add(time.Hour * 24 * -365)
	for _, meta := range doc.Find("meta").Nodes {
		for _, attr := range meta.Attr {
			if t, err := dateparse.ParseAny(attr.Val); err == nil && t.After(yearBefore) {
				return t
			}
		}
	}
	times := r.Headers.Values("Last-Modified")
	for _, t := range times {
		if tt, err := time.Parse(time.RFC1123, t); err == nil {
			return tt
		}
	}
	times = r.Headers.Values("Date")
	for _, t := range times {
		if tt, err := time.Parse(time.RFC1123, t); err == nil {
			return tt
		}
	}
	return time.Now()
}

func (s *Spider) Listener() {
	for info := range s.dataCh {
		if err := s.Queue.Produce(&info); err != nil {
			zl.Error().Err(fmt.Errorf("can't produce message for '%s': %s", info.Url, err))
		}
		logger := zl.With().Str("Title", info.Source.Title).Str("URL", info.Url).Time("Date", info.Source.Date).Logger()
		logger.Info().Msgf("Document sent")
		logger.Debug().Str("Content", info.Data).Msg("Document content sent")

	}
}

func (s *Spider) Close() {
	close(s.dataCh)
}

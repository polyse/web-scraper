package spider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/extractor"
	"github.com/polyse/web-scraper/internal/locker"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	zl "github.com/rs/zerolog/log"
	"go.uber.org/ratelimit"
)

type Spider struct {
	Queue              *rabbitmq.Queue
	RateLimit          ratelimit.Limiter
	Delay, RandomDelay time.Duration
	dataCh             chan sdk.RawData
	locker             *locker.Conn
}

func NewSpider(queue *rabbitmq.Queue, limit int, delay, randomDelay time.Duration, locker *locker.Conn) (*Spider, error) {
	s := &Spider{
		Queue:       queue,
		RateLimit:   ratelimit.New(limit),
		Delay:       delay,
		RandomDelay: randomDelay,
		dataCh:      make(chan sdk.RawData),
		locker:      locker,
	}
	go s.Listener()
	return s, nil
}

func (s *Spider) Scrap(ctx context.Context, startUrl *url.URL) error {
	co, err := s.initScrapper(ctx, startUrl)
	if err != nil {
		return err
	}
	if err := co.Visit(startUrl.String()); err != nil {
		return err
	}
	go func() {
		co.Wait()
		zl.Debug().Msgf("%s", co.String())
		zl.Debug().Msgf("Finish %v", startUrl)
	}()
	return nil
}

func (s *Spider) initScrapper(ctx context.Context, u *url.URL) (*colly.Collector, error) {
	co := colly.NewCollector(
		colly.Async(true),
		colly.AllowedDomains(u.Host),
	)
	extensions.RandomUserAgent(co)
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
		// check if url is locked
		err := lock(s.locker, fullLink)
		if err == errUrlIsLocked {
			return
		}
		if err != nil {
			zl.Debug().Err(err).Str("URL", fullLink).Msg("Failed to lock url")
			return
		}
		zl.Debug().Str("URL", fullLink).Msg("Url is locked")
		s.RateLimit.Take()
		err = e.Request.Visit(fullLink)
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
			unlock(s.locker, r.Request.URL.String())
			return
		}

		title := doc.Find("Title").Text()
		title = strings.TrimSpace(title)
		if title == "" {
			return
		}

		actual, err := extractor.ExtractContentFromHTML(payload)
		if err != nil {
			zl.Debug().Err(err).Msgf("Can't parse")
			unlock(s.locker, r.Request.URL.String())
			return
		}
		content := extractor.Clean(actual)

		t := s.searchDate(r, doc)

		s.dataCh <- sdk.RawData{
			Source: sdk.Source{
				Date:  t,
				Title: title,
			},
			Url:  r.Request.URL.String(),
			Data: content,
		}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msgf("Can't connect to URL %s", filepath.Join(r.Request.URL.Host, r.Request.URL.Path))
		unlock(s.locker, r.Request.URL.String())
	})
	return co, nil
}

func (s *Spider) searchDate(r *colly.Response, doc *goquery.Document) time.Time {
	yearBefore := time.Now().Add(time.Hour * 24 * -365)
	//TODO: rewrite the hack below
	for _, meta := range doc.Find("meta").Nodes {
		for _, attr := range meta.Attr {
			if strings.Contains(attr.Val, "published_time") {
				for _, dateAttr := range meta.Attr {
					if dateAttr.Key == "content" {
						if t, err := dateparse.ParseAny(dateAttr.Val); err == nil && t.After(yearBefore) && t.Before(time.Now()) {
							return t
						}
					}
				}
			}
		}

		for _, attr := range meta.Attr {
			if t, err := dateparse.ParseAny(attr.Val); err == nil && t.After(yearBefore) && t.Before(time.Now()) {
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
		logger := zl.With().Str("Title", info.Source.Title).Str("URL", info.Url).Time("Date", info.Source.Date).Logger()
		if err := s.Queue.Produce(&info); err != nil {
			logger.Error().Msg("can't produce message")
		}
		logger.Info().Msgf("Document sent")

	}
}

func (s *Spider) Close() {
	close(s.dataCh)
}

var (
	errUrlIsLocked = errors.New("url is locked")
)

// Try to lock url
func lock(l *locker.Conn, url string) error {
	locked, err := l.TryLock(url)
	if err != nil {
		return fmt.Errorf("failed to lock url: %w", err)
	}
	if !locked {
		return errUrlIsLocked
	}
	return nil
}

// Try to unlock url
func unlock(l *locker.Conn, url string) {
	unlocked, err := l.Unlock(url)
	if err != nil {
		err := fmt.Errorf("failed to unlock url: %w", err)
		zl.Debug().Err(err).Str("URL", url).Msg("Failed to unlock url")
	}
	if !unlocked {
		zl.Debug().Str("URL", url).Msg("Link wasn't locked")
	}
}

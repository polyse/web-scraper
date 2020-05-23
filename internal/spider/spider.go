package spider

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/extractor"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	zl "github.com/rs/zerolog/log"
	"go.uber.org/ratelimit"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh             chan sdk.RawData
	currentDomain      string
	Queue              *rabbitmq.Queue
	RateLimit          ratelimit.Limiter
	Delay, RandomDelay time.Duration
}

func NewSpider(queue *rabbitmq.Queue, limit int, delay, randomDelay time.Duration) (*Spider, error) {
	s := &Spider{
		DataCh:        make(chan sdk.RawData),
		currentDomain: "",
		Queue:         queue,
		RateLimit:     ratelimit.New(limit),
		Delay:         delay,
		RandomDelay:   randomDelay,
	}
	go s.Listener()
	return s, nil
}

func (s *Spider) StartSearch(startUrl string) {
	u, err := url.Parse(startUrl)
	if err != nil {
		zl.Warn().Err(err).Msgf("Can't visit page : %v", startUrl)
		return
	}
	co := s.initScrapper(u)
	err = co.Visit(startUrl)
	go func() {
		for {
			<-time.After(5 * time.Second)
			co.UserAgent = surferua.New().String()
		}
	}()
	if err != nil {
		zl.Warn().Err(err).Msgf("Can't visit page : %v", startUrl)
	}
	co.Wait()
	zl.Debug().Msgf("%s", co.String())
	zl.Debug().Msgf("Finish %v", startUrl)
}

func (s *Spider) initScrapper(u *url.URL) *colly.Collector {
	co := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent(surferua.New().String()),
		colly.AllowedDomains(u.Host),
	)
	err := co.Limit(&colly.LimitRule{
		Parallelism: runtime.NumCPU(),
		Delay:       s.Delay,
		RandomDelay: s.RandomDelay,
	})
	if err != nil {
		zl.Warn().Err(err).Msg("Can't set limit")
	}
	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fullLink := e.Request.AbsoluteURL(link)
		zl.Debug().Msgf("Find URL : %v", fullLink)
		s.RateLimit.Take()
		err := e.Request.Visit(fullLink)
		if err == colly.ErrAlreadyVisited || err == colly.ErrForbiddenDomain {
			return
		}
		if err != nil {
			zl.Warn().Err(err).Msgf("Can't visit page : %v", fullLink)
		}
	})
	co.OnResponse(func(r *colly.Response) {
		payload := string(r.Body[:])
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload))
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}
		title := doc.Find("Title").Text()
		actual, err := extractor.ExtractContentFromHTML(payload)
		var content string
		if err != nil {
			zl.Debug().Err(err).Msgf("Can't parse")
		} else {
			content = extractor.Clean(actual)
		}
		var pageTime string
		times := r.Headers.Values("Last-Modified")
		if len(times) == 0 {
			times = r.Headers.Values("Date")
			if len(times) > 0 {
				pageTime = times[0]
			}
		} else {
			pageTime = times[0]
		}
		t, err := time.Parse(time.RFC1123, pageTime)
		if err != nil {
			t = time.Time{}
		}
		s.DataCh <- sdk.RawData{
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
	return co
}

func (s *Spider) Listener() {
	for info := range s.DataCh {
		if err := s.Queue.Produce(&info); err != nil {
			zl.Error().Err(fmt.Errorf("Can't produce message for '%s': %s", info.Url, err))
		}
		zl.Info().Str("Title", info.Title).Str("URL", info.Url).Msgf("Document sent")
	}
}

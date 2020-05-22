package spider

import (
	"fmt"
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
	DataCh        chan sdk.RawData
	currentDomain string
	Queue         *rabbitmq.Queue
	RateLimit     ratelimit.Limiter
}

func NewSpider(queue *rabbitmq.Queue, limit int) (*Spider, error) {
	s := &Spider{
		DataCh:        make(chan sdk.RawData),
		currentDomain: "",
		Queue:         queue,
		RateLimit:     ratelimit.New(limit),
	}
	go s.Listener()
	return s, nil
}

func (s *Spider) StartSearch(domain string) {
	co := s.initScrapper()
	err := co.Visit(domain)
	if err != nil {
		zl.Warn().Err(err).Msgf("Can't visit page : %v", domain)
	}
	co.Wait()
	zl.Debug().Msgf("%s", co.String())
	zl.Debug().Msgf("Finish %v", domain)
}

func (s *Spider) initScrapper() *colly.Collector {
	co := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent(surferua.New().String()),
	)
	err := co.Limit(&colly.LimitRule{
		Parallelism: runtime.NumCPU(),
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
		zl.Debug().Msgf("Message for '%s' produced", info.Url)
	}
}

package spider

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/extractor"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh        chan sdk.RawData
	currentDomain string
	Queue         *rabbitmq.Queue
}

func NewSpider(queue *rabbitmq.Queue) (*Spider, error) {
	m := &Spider{
		DataCh:        make(chan sdk.RawData),
		currentDomain: "",
		Queue:         queue,
	}
	go m.Listener()
	return m, nil
}

func (m *Spider) StartSearch(domain string) {
	co := initScrapper(m.DataCh)
	err := co.Visit(domain)
	if err != nil {
		zl.Warn().Err(err).Msgf("Can't visit page : %v", domain)
	}
	co.Wait()
	zl.Debug().Msgf("Finish %v", domain)
}

func initScrapper(dataCh chan sdk.RawData) *colly.Collector {
	co := colly.NewCollector(
		colly.Async(true),
		colly.UserAgent(surferua.New().String()),
	)
	err := co.Limit(&colly.LimitRule{
		Parallelism: 1,
		Delay:       5 * time.Second,
	})
	if err != nil {
		zl.Warn().Err(err).Msg("Can't set limit")
	}
	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fullLink := e.Request.AbsoluteURL(link)
		zl.Debug().Msgf("Find URL : %v", fullLink)
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
		dataCh <- sdk.RawData{
			Source: sdk.Source{
				Date:  t,
				Title: title,
			},
			Url:  filepath.Join(r.Request.URL.Host, r.Request.URL.Path),
			Data: content,
		}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msg("Can't connect to URL")
	})
	return co
}

func (m *Spider) Listener() {
	for info := range m.DataCh {
		if err := m.Queue.Produce(&info); err != nil {
			zl.Error().Err(fmt.Errorf("Can't produce message for '%s': %s", info.Url, err))
		}
		zl.Debug().Msgf("Message for '%s' produced", info.Url)
	}
}

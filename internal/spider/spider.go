package spider

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	database_sdk "github.com/polyse/database-sdk"

	"github.com/polyse/web-scraper/internal/extractor"

	"github.com/polyse/web-scraper/internal/rabbitmq"

	"github.com/PuerkitoBio/goquery"
	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh        chan database_sdk.RawData
	mutex         *sync.Mutex
	currentDomain string
	Queue         *rabbitmq.Queue
}

func NewSpider(queue *rabbitmq.Queue) (*Spider, error) {
	m := &Spider{
		DataCh:        make(chan database_sdk.RawData),
		mutex:         &sync.Mutex{},
		currentDomain: "",
		Queue:         queue,
	}
	return m, nil
}

func (m *Spider) Colly(domain string) {
	m.currentDomain = domainutil.Domain(domain)
	go m.Listener()
	m.collyScrapper(domain)
	close(m.DataCh)
	m.mutex.Lock()
	zl.Debug().
		Msgf("Finish %v and start new", domain)
	m.DataCh = make(chan database_sdk.RawData)
	m.mutex.Unlock()
}

func (m *Spider) collyScrapper(URL string) {
	zl.Debug().Msgf("%v", m.currentDomain)
	co := colly.NewCollector(
		colly.AllowedDomains(m.currentDomain),
		colly.Async(true),
		colly.UserAgent(surferua.New().String()),
	)

	co.Limit(&colly.LimitRule{
		Parallelism: 4,
		RandomDelay: 10 * time.Second,
	})

	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fullLink := e.Request.AbsoluteURL(link)
		zl.Debug().Msgf("Find URL : %v", fullLink)
		e.Request.Visit(fullLink)
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
		if err != nil {
			zl.Debug().Err(err).Msgf("Can't parse")
		}
		content := extractor.Clean(actual)
		times := r.Headers.Values("Last-Modified")
		if len(times) == 0 {
			times = r.Headers.Values("Date")
		}
		t, err := time.Parse(time.RFC1123, times[0])
		if err != nil {
			t = time.Time{}
		}
		m.DataCh <- database_sdk.RawData{
			Source: database_sdk.Source{
				Date:  t,
				Title: title,
			},
			Url:  filepath.Join(r.Request.URL.Host, r.Request.URL.Path),
			Data: content}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msg("Can't connect to URL")
		return
	})
	co.Visit(URL)
	co.Wait()
}

func (m *Spider) Listener() {
	defer m.mutex.Unlock()
	m.mutex.Lock()
	for info := range m.DataCh {
		if err := m.Queue.Produce(&info); err != nil {
			zl.Error().Err(fmt.Errorf("cannot produce message for '%s': %s", m.currentDomain, err))
		} else {
			zl.Debug().Msgf("Message for '%s' produced: %v", m.currentDomain, info)
		}
	}
}

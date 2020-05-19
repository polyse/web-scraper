package spider

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polyse/web-scraper/internal/extractor"

	"github.com/polyse/web-scraper/internal/rabbitmq"

	"github.com/PuerkitoBio/goquery"
	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh        chan rabbitmq.Message
	mutex         *sync.Mutex
	currentDomain string
	Queue         *rabbitmq.Queue
}

func NewSpider(queue *rabbitmq.Queue) (*Spider, error) {
	m := &Spider{
		DataCh:        make(chan rabbitmq.Message),
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
	m.DataCh = make(chan rabbitmq.Message)
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
		RandomDelay: 1 * time.Second,
	})

	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fullLink := e.Request.AbsoluteURL(link)
		zl.Debug().Msgf("Find URL : %v", fullLink)
		e.Request.Visit(fullLink)
	})

	t := time.Time{}

	co.OnHTML("dateformat", func(e *colly.HTMLElement) {
		tm := e.Attr("time")
		date, _ := strconv.ParseInt(tm, 10, 64)
		t = time.Unix(date, 0)
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
		/*d, err := readability.NewDocument(payload)
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}
		content := d.Content()*/

		actual, err := extractor.ExtractContentFromHTML(payload)
		if err != nil {
			zl.Debug().Err(err).Msgf("Can't parse")
		}
		content := extractor.Clean(actual)
		m.DataCh <- rabbitmq.Message{
			Source: rabbitmq.Source{
				Date:  &t,
				Title: title,
			},
			Url:  URL,
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

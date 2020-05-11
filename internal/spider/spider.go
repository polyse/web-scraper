package spider

import (
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	"github.com/mauidude/go-readability"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh        chan SitesInfo
	mutex         *sync.Mutex
	currentDomain string
	info          []SitesInfo
}

type SitesInfo struct {
	Title   string
	URL     string
	Payload string
}

func NewSpider() (*Spider, error) {
	m := &Spider{
		DataCh:        make(chan SitesInfo),
		mutex:         &sync.Mutex{},
		currentDomain: "",
		info:          []SitesInfo{},
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
	m.DataCh = make(chan SitesInfo)
	m.mutex.Unlock()
	zl.Debug().
		Msgf("%v", m.info)
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

	co.OnResponse(func(r *colly.Response) {
		payload := string(r.Body[:])
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload))
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}
		title := doc.Find("Title").Text()
		d, err := readability.NewDocument(payload)
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}
		content := d.Content()
		m.DataCh <- SitesInfo{title, URL, content}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msg("Can't connect to URL")
		m.DataCh <- SitesInfo{"", URL, err.Error()}
		return
	})
	co.Visit(URL)
	co.Wait()
}

func (m *Spider) Listener() {
	defer m.mutex.Unlock()
	m.mutex.Lock()
	for info := range m.DataCh {
		m.info = append(m.info, info)
	}
}

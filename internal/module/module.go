package module

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Module struct {
	outPath       string
	DataCh        chan SitesInfo
	mutex         *sync.Mutex
	currentDomain string
	info          []SitesInfo `json:"info"`
}

type SitesInfo struct {
	Title   string `json:"Title"`
	URL     string `json:"URL"`
	Payload string `json:"Payload"`
}

func NewModule(outPath string) (*Module, error) {
	m := &Module{
		outPath:       outPath,
		DataCh:        make(chan SitesInfo),
		mutex:         &sync.Mutex{},
		currentDomain: "",
		info:          []SitesInfo{},
	}
	return m, nil
}

func (m *Module) WriteResult() {
	file, _ := os.OpenFile(m.outPath, os.O_CREATE, os.ModePerm)
	defer file.Close()

	encoder := json.NewEncoder(file)
	err := encoder.Encode(m.info)

	if err != nil {
		zl.Warn().Err(err).
			Msg("Can't write json in file")
	}
	zl.Debug().
		Msg("Write data")
}

func (m *Module) Colly(domain string) {
	m.currentDomain = domainutil.Domain(domain)
	go m.Listener()
	m.collyScrapper(domain)
	close(m.DataCh)
	m.mutex.Lock()
	zl.Debug().
		Msgf("Finish %v and start new", domain)
	m.DataCh = make(chan SitesInfo)
	m.mutex.Unlock()
	m.WriteResult()
}

func (m *Module) collyScrapper(URL string) {
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

	// On every a element which has href attribute call callback
	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Visit link found on page on a new thread
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
		/*doc, err := readability.NewDocument(Payload)
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			return
		}
		content := doc.Content()*/
		m.DataCh <- SitesInfo{title, URL, payload}
	})

	// Set error handler
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msg("Can't connect to URL")
		m.DataCh <- SitesInfo{"", URL, err.Error()}
		return
	})
	// Start scraping
	co.Visit(URL)
	// Wait until threads are finished
	co.Wait()
}

func (m *Module) Listener() {
	defer m.mutex.Unlock()
	m.mutex.Lock()
	for info := range m.DataCh {
		m.info = append(m.info, info)
	}
}

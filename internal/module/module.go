package module

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	zl "github.com/rs/zerolog/log"
)

type Module struct {
	outPath string
	DataCh  chan []string
	mutex   *sync.Mutex
	// Domains -> URLs -> Infos
	SitesInfo map[string]map[string][]string
}

func NewModule(filePath string, outPath string) (*Module, error) {
	m := &Module{
		outPath:   outPath,
		DataCh:    make(chan []string),
		mutex:     &sync.Mutex{},
		SitesInfo: map[string]map[string][]string{},
	}
	m.ReadFilePath(filePath)
	return m, nil
}

func (m *Module) ReadFilePath(filePath string) {
	var listOfFiles = []string{}
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		zl.Fatal().Err(err).
			Msgf("Can't open input file %v", filePath)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		listOfFiles = append(listOfFiles, scanner.Text())
	}
	zl.Debug().
		Msgf("IN: %v", listOfFiles)
	for _, file := range listOfFiles {
		m.SitesInfo[file] = map[string][]string{}
	}
}

func (m *Module) WriteResult() {
	file, _ := os.OpenFile(m.outPath, os.O_CREATE, os.ModePerm)
	defer file.Close()

	encoder := json.NewEncoder(file)
	err := encoder.Encode(&m.SitesInfo)

	if err != nil {
		zl.Warn().Err(err).
			Msg("Can't write json in file")
	}
	zl.Debug().
		Msg("Write data")
}

func (m *Module) Colly() {
	for domain, _ := range m.SitesInfo {
		go m.Listener(domain)
		m.collyScrapper(domain)
		close(m.DataCh)
		m.mutex.Lock()
		zl.Debug().
			Msgf("Finish %v and start new", domain)
		m.DataCh = make(chan []string)
		m.mutex.Unlock()
	}
	m.WriteResult()
}

func (m *Module) collyScrapper(URL string) {
	zl.Debug().
		Msgf("Got correct URL: %v ", URL)
	// Instantiate default collector
	co := colly.NewCollector(
		// MaxDepth is 2, so only the links on the scraped page
		// and links on those pages are visited
		colly.MaxDepth(2),
		colly.Async(true),
		colly.UserAgent("Mozilla/5.0 (Windows NT 6.1; Win64; x64; rv:47.0) Gecko/20100101 Firefox/47.0"),
	)

	// Limit the maximum parallelism to 2
	// This is necessary if the goroutines are dynamically
	// created to control the limit of simultaneous requests.
	//
	// Parallelism can be controlled also by spawning fixed
	// number of go routines.
	co.Limit(&colly.LimitRule{Parallelism: 3})
	// On every a element which has href attribute call callback
	co.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Visit link found on page on a new thread
		fullLink := e.Request.AbsoluteURL(link)
		e.Request.Visit(fullLink)
	})

	co.OnHTML("p", func(e *colly.HTMLElement) {
		zl.Debug().
			Msgf("Find: %v\ntext: %v", URL, e.DOM.Text())
		if e.DOM.Text() == "" {
			return
		}
		m.DataCh <- []string{URL, e.DOM.Text()}
	})

	// Set error handler
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().
			Msgf("Request URL: %v\nError: %v", r.Request.URL, err)
	})
	// Start scraping
	co.Visit(URL)
	// Wait until threads are finished
	co.Wait()
}

func (m *Module) Listener(domain string) {
	defer m.mutex.Unlock()
	m.mutex.Lock()
	for newElem := range m.DataCh {
		zl.Debug().
			Msgf("Got %v", newElem)
		if _, ok := m.SitesInfo[domain][newElem[0]]; !ok {
			m.SitesInfo[domain][newElem[0]] = []string{newElem[1]}
		} else {
			m.SitesInfo[domain][newElem[0]] = append(m.SitesInfo[domain][newElem[0]], newElem[1])
		}
		zl.Debug().
			Msgf("Add")
	}
}

func (m *Module) Go() {
	for domain, _ := range m.SitesInfo {
		go m.Listener(domain)
		m.GoQuery(domain, 2)
		close(m.DataCh)
		m.mutex.Lock()
		zl.Debug().
			Msgf("Finish %v and start new", domain)
		m.DataCh = make(chan []string)
		m.mutex.Unlock()
	}
	m.WriteResult()
}

func (m *Module) GoQuery(URL string, depthLevel int) {
	if depthLevel == 0 {
		return
	}

	zl.Debug().
		Msgf("Got URL: %v ", URL)

	// Request the HTML page.
	res, err := http.Get(URL)
	if err != nil {
		zl.Debug().Err(err).
			Msgf("Can't get url %v", URL)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		zl.Debug().Err(err).
			Msgf("status code error: %d %s", res.StatusCode, res.Status)
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		zl.Debug().Err(err).
			Msg("Can't load document")
		return
	}

	doc.Find("p").Each(func(_ int, s *goquery.Selection) {
		text := s.Nodes[0].Data

		if text == "" {
			return
		} else if text == "\n" {
			return
		} else {
			fmt.Println(text)
			m.DataCh <- []string{URL, text}
		}
	})
	// Find the review items
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		link, _ := s.Attr("href")
		zl.Debug().
			Msgf("Find : %v", link)
		//go m.GoQuery(link, depthLevel-1)
	})
	zl.Debug().
		Msgf("Finish the %v", URL)
}

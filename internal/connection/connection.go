package connection

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	zl "github.com/rs/zerolog/log"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Connection struct {
	server *http.Server
	addr   string
	Files  []string
	dataCh chan []string
	mutex  *sync.Mutex
	// Domens -> URLs -> Infos
	SitesInfo map[string]map[string][]string
}

var RegExp *regexp.Regexp

func New(addr string) (*Connection, error) {
	mux := &http.ServeMux{}
	siteHandler := logMiddleware(mux)
	c := &Connection{
		server: &http.Server{
			Addr:         addr,
			Handler:      siteHandler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		addr:      addr,
		Files:     []string{},
		dataCh:    make(chan []string),
		mutex:     &sync.Mutex{},
		SitesInfo: map[string]map[string][]string{},
	}
	mux.HandleFunc("/colly", c.collyHandler)
	mux.HandleFunc("/go", c.goQueryHandler)
	return c, nil
}

// logMiddleware logging all request
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		zl.Debug().
			Str("method", r.Method).
			Str("remote", r.RemoteAddr).
			Str("path", r.URL.Path).
			Int("duration", int(time.Since(start))/int(math.Pow10(9))).
			Msgf("Called url %s", r.URL.Path)
	})
}

func (c *Connection) collyHandler(w http.ResponseWriter, r *http.Request) {
	for _, domen := range c.Files {
		go c.listener(domen)
		c.collyScrapper(domen)
		close(c.dataCh)
		c.mutex.Lock()
		zl.Debug().Msgf("Finish %v and start new", domen)
		c.dataCh = make(chan []string)
		c.mutex.Unlock()
	}
	// dump results
	b, err := json.Marshal(c.SitesInfo)
	if err != nil {
		zl.Warn().Err(err).
			Msg("failed to serialize response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	statusCode, err := w.Write(b)
	if err != nil {
		zl.Warn().Err(err).
			Msgf("failed to write : %v ", statusCode)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	zl.Debug().
		Msg("Send data!")
}

func (c *Connection) goQueryHandler(w http.ResponseWriter, r *http.Request) {
	for _, domen := range c.Files {
		go c.listener(domen)
		c.goQuery(domen, "https://"+domen, 2)
		close(c.dataCh)
		c.mutex.Lock()
		zl.Debug().Msgf("Finish %v and start new", domen)
		c.dataCh = make(chan []string)
		c.mutex.Unlock()
	}
	// dump results
	b, err := json.Marshal(c.SitesInfo)
	if err != nil {
		zl.Warn().Err(err).
			Msg("failed to serialize response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	statusCode, err := w.Write(b)
	if err != nil {
		zl.Warn().Err(err).
			Msgf("failed to write : %v ", statusCode)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	zl.Debug().
		Msg("Send data!")
}

func (c *Connection) collyScrapper(URL string) {
	submatchall := RegExp.FindAllString(URL, -1)
	correctURL := strings.Join(submatchall, "")
	zl.Debug().
		Msgf("Got correct URL: %v ", correctURL)
	// Instantiate default collector
	co := colly.NewCollector(
		colly.AllowedDomains(correctURL, "m."+correctURL, "www."+correctURL),
		// MaxDepth is 2, so only the links on the scraped page
		// and links on those pages are visited
		colly.MaxDepth(2),
		colly.Async(true),
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
		c.dataCh <- []string{URL, e.DOM.Text()}
	})

	// Set error handler
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().
			Msgf("Request URL: %v\nError: %v", r.Request.URL, err)
	})
	// Start scraping
	co.Visit("https://" + URL)
	// Wait until threads are finished
	co.Wait()
}

func (c *Connection) listener(domen string) {
	defer c.mutex.Unlock()
	c.mutex.Lock()
	c.SitesInfo[domen] = map[string][]string{}
	for newElem := range c.dataCh {
		zl.Debug().Msgf("Got %v", newElem)
		if _, ok := c.SitesInfo[domen][newElem[0]]; !ok {
			c.SitesInfo[domen][newElem[0]] = []string{newElem[1]}
		} else {
			c.SitesInfo[domen][newElem[0]] = append(c.SitesInfo[domen][newElem[0]], newElem[1])
		}
	}
}

func (c *Connection) isAlreadyRead(URL string) bool {
	for _, domenInfo := range c.SitesInfo {
		if _, ok := domenInfo[URL]; ok {
			return true
		}
	}
	return false
}

func (c *Connection) goQuery(domen, URL string, depthLevel int) {
	if depthLevel == 0 {
		return
	}
	if c.isAlreadyRead(URL) == true {
		return
	}
	zl.Debug().Msgf("Got URL: %v ", URL)
	// Check domen
	if !strings.Contains(URL, domen) {
		return
	}
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
		var text string
		if s.Nodes[0].FirstChild.Data != "" {
			text = s.Nodes[0].FirstChild.Data
		} else if s.Nodes[0].FirstChild.Attr[0].Val != "" {
			text = s.Nodes[0].FirstChild.Attr[0].Val
		} else if s.Nodes[0].Data != "" {
			text = s.Nodes[0].Data
		}

		if text == "" {
			return
		} else {
			zl.Debug().
				Msgf("Find : %v\n text : %v", URL, text)
			c.dataCh <- []string{URL, text}
		}
	})
	// Find the review items
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		link, _ := s.Attr("href")
		if strings.Contains(link, "http") == false {
			link = "https://" + domen + link
		}
		//zl.Debug().
		//	Msgf("Find : %v", link)
		go c.goQuery(domen, link, depthLevel-1)
	})
	zl.Debug().
		Msgf("Finish the %v", URL)
}

func (c *Connection) Start() {
	RegExp = regexp.MustCompile(`^(([a-zA-Z]{1})|([a-zA-Z]{1}[a-zA-Z]{1})|([a-zA-Z]{1}[0-9]{1})|([0-9]{1}[a-zA-Z]{1})|([a-zA-Z0-9][a-zA-Z0-9-_]{1,61}[a-zA-Z0-9]))\.([a-zA-Z]{2,6}|[a-zA-Z0-9-]{2,30}\.[a-zA-Z]{2,3})$`)
	zl.Debug().
		Msgf("listening on %v", c.addr)
	err := c.server.ListenAndServe()
	if err != nil {
		zl.Fatal().Err(err).
			Msg("Can't start service")
	}
}

func (c *Connection) Close() error {
	return c.server.Close()
}

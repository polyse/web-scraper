package spider

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/polyse/web-scraper/internal/locker"

	"github.com/polyse/web-scraper/internal/rabbitmq"

	"github.com/PuerkitoBio/goquery"
	"github.com/bobesa/go-domain-util/domainutil"
	"github.com/gocolly/colly/v2"
	"github.com/mauidude/go-readability"
	zl "github.com/rs/zerolog/log"
	"go.zoe.im/surferua"
)

type Spider struct {
	DataCh        chan rabbitmq.Message
	mutex         *sync.Mutex
	currentDomain string
	queue         *rabbitmq.Queue
	l             *locker.Conn
}

func NewSpider(queue *rabbitmq.Queue, l *locker.Conn) (*Spider, error) {
	m := &Spider{
		DataCh:        make(chan rabbitmq.Message),
		mutex:         &sync.Mutex{},
		currentDomain: "",
		queue:         queue,
		l:             l,
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
	// check if url is locked
	err := lock(m.l, URL)
	if err != nil {
		zl.Debug().Err(err).Str("URL", URL).Msg("Failed to lock url")
		return
	}
	zl.Debug().Str("URL", URL).Msg("Url is locked")

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
		// check if url is locked
		err := lock(m.l, fullLink)
		if err != nil {
			zl.Debug().Err(err).Str("URL", fullLink).Msg("Failed to lock url")
			return
		}
		zl.Debug().Str("URL", fullLink).Msg("Url is locked")
		e.Request.Visit(fullLink)
	})

	co.OnResponse(func(r *colly.Response) {
		payload := string(r.Body[:])
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload))
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			unlock(m.l, r.Request.URL.String())
			return
		}
		title := doc.Find("Title").Text()
		d, err := readability.NewDocument(payload)
		if err != nil {
			zl.Debug().Err(err).
				Msg("Can't load html text")
			unlock(m.l, r.Request.URL.String())
			return
		}
		content := d.Content()
		m.DataCh <- rabbitmq.Message{Source: title, Url: URL, Data: content}
	})
	co.OnError(func(r *colly.Response, err error) {
		zl.Debug().Err(err).Msg("Can't connect to URL")
		unlock(m.l, r.Request.URL.String())
		return
	})
	co.Visit(URL)
	co.Wait()
}

func (m *Spider) Listener() {
	defer m.mutex.Unlock()
	m.mutex.Lock()
	for info := range m.DataCh {
		if err := m.queue.Produce(&info); err != nil {
			zl.Error().Err(fmt.Errorf("cannot produce message for '%s': %s", m.currentDomain, err))
		} else {
			zl.Debug().Msgf("Message for '%s' produced: %v", m.currentDomain, info)
		}
	}
}

// Try to lock url
func lock(l *locker.Conn, url string) error {
	locked, err := l.TryLock(url)
	if err != nil {
		return fmt.Errorf("failed to lock url: %w", err)
	}
	if !locked {
		return errors.New("url is locked")
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

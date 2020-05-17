package api

import (
	"net/http"

	"github.com/polyse/web-scraper/internal/broker"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/polyse/web-scraper/internal/spider"
	zl "github.com/rs/zerolog/log"
)

type API struct {
	s    *spider.Spider
	e    *echo.Echo
	b    *broker.Broker
	addr string
}

func healthcheck(e echo.Context) error {
	return e.String(http.StatusOK, "Service start")
}

func (a *API) collyHandler(e echo.Context) error {
	domain := e.FormValue("domain")
	go a.s.Colly(domain)
	return e.String(http.StatusAccepted, domain)
}

func New(addr string, s *spider.Spider, b *broker.Broker) (*API, error) {
	e := echo.New()
	a := &API{
		s:    s,
		e:    e,
		b:    b,
		addr: addr,
	}
	e.Use(middleware.Logger())
	e.GET("/healthcheck", healthcheck)
	e.POST("/colly", a.collyHandler)
	return a, nil
}

func (a *API) Start() {
	zl.Debug().
		Msgf("listening on %v", a.addr)
	err := a.e.Start(a.addr)
	if err != nil {
		zl.Fatal().Err(err).
			Msg("Can't start service")
	}
	a.b.StartBroker()
}

func (a *API) Close() error {
	return a.e.Close()
}

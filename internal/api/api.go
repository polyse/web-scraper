package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/polyse/web-scraper/internal/spider"
	zl "github.com/rs/zerolog/log"
)

type API struct {
	s    *spider.Spider
	e    *echo.Echo
	addr string
}

func healthcheck(e echo.Context) error {
	return e.String(http.StatusOK, "Service start")
}

func (a *API) collyHandler(e echo.Context) error {
	domain := e.FormValue("domain")
	go a.s.StartSearch(domain)
	return e.String(http.StatusAccepted, domain)
}

func New(addr, auth string, s *spider.Spider) (*API, error) {
	e := echo.New()
	a := &API{
		s:    s,
		e:    e,
		addr: addr,
	}
	e.Use(middleware.Logger())
	e.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		return key == auth, nil
	}))
	e.GET("/healthcheck", healthcheck)
	e.POST("/colly", a.collyHandler)
	return a, nil
}

func (a *API) Start() error {
	zl.Debug().
		Msgf("listening on %v", a.addr)
	return a.e.Start(a.addr)
}

func (a *API) Close() error {
	close(a.s.DataCh)
	return a.e.Close()
}

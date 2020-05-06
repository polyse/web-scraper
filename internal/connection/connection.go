package connection

import (
	"net/http"

	"github.com/labstack/echo/v4/middleware"

	"github.com/labstack/echo/v4"
	"github.com/polyse/web-scraper/internal/module"
	zl "github.com/rs/zerolog/log"
)

type Connection struct {
	module *module.Module
	e      *echo.Echo
	addr   string
}

func healthcheck(e echo.Context) error {
	return e.String(http.StatusOK, "Service start")
}

func (c *Connection) collyHandler(e echo.Context) error {
	domain := e.FormValue("domain")
	go c.module.Colly(domain)
	return e.String(http.StatusAccepted, domain)
}

func New(addr string, module *module.Module) (*Connection, error) {
	e := echo.New()
	c := &Connection{
		module: module,
		e:      e,
		addr:   addr,
	}
	e.Use(middleware.Logger())
	e.GET("/healthcheck", healthcheck)
	e.POST("/colly", c.collyHandler)
	return c, nil
}

func (c *Connection) Start() {
	zl.Debug().
		Msgf("listening on %v", c.addr)
	err := c.e.Start(c.addr)
	if err != nil {
		zl.Fatal().Err(err).
			Msg("Can't start service")
	}
}

func (c *Connection) Close() error {
	return c.e.Close()
}

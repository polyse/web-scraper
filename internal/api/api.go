package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	zl "github.com/rs/zerolog/log"

	"github.com/polyse/web-scraper/internal/spider"
)

type Context struct {
	echo.Context
	Ctx context.Context
}

type API struct {
	s    *spider.Spider
	e    *echo.Echo
	addr string
}

func healthcheck(e echo.Context) error {
	return e.String(http.StatusOK, "ok")
}

func (a *API) collyHandler(e echo.Context) error {
	startUrl, err := url.Parse(e.FormValue("url"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "url is malformed")
	}
	cc := e.(*Context)
	if err := a.s.Scrap(cc.Ctx, startUrl); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return e.JSON(http.StatusAccepted, map[string]string{"started": startUrl.String()})
}

func New(ctx context.Context, addr, auth string, s *spider.Spider) (*API, error) {
	e := echo.New()
	a := &API{
		s:    s,
		e:    e,
		addr: addr,
	}
	// inject global cancellable context to every request
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &Context{
				Context: c,
				Ctx:     ctx,
			}
			return next(cc)
		}
	})
	e.Use(logger())
	e.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		return key == auth, nil
	}))
	e.GET("/healthcheck", healthcheck)
	e.POST("/start", a.collyHandler)
	return a, nil
}

func logger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()
			start := time.Now()

			err := next(c)
			stop := time.Now()

			zl.Debug().
				Str("remote", req.RemoteAddr).
				Str("user_agent", req.UserAgent()).
				Str("method", req.Method).
				Str("path", c.Path()).
				Int("status", res.Status).
				Dur("duration", stop.Sub(start)).
				Str("duration_human", stop.Sub(start).String()).
				Msgf("called url %s", req.URL)

			return err
		}
	}
}

func (a *API) Start() error {
	zl.Debug().
		Msgf("listening on %v", a.addr)
	err := a.e.Start(a.addr)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *API) Close() error {
	return a.e.Close()
}

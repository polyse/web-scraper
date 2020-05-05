package connection

import (
	"net/http"
	"time"

	"github.com/polyse/web-scraper/internal/module"
	zl "github.com/rs/zerolog/log"
)

type Connection struct {
	module *module.Module
	server *http.Server
	addr   string
}

func New(addr string, module *module.Module) (*Connection, error) {
	mux := &http.ServeMux{}
	siteHandler := logMiddleware(mux)
	c := &Connection{
		module: module,
		server: &http.Server{
			Addr:         addr,
			Handler:      siteHandler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		addr: addr,
	}
	mux.HandleFunc("/colly", c.collyHandler)
	mux.HandleFunc("/go", c.goQueryHandler)
	return c, nil
}

func (c *Connection) collyHandler(w http.ResponseWriter, _ *http.Request) {
	go c.module.Colly()
	w.WriteHeader(http.StatusAccepted)
	w.Header().Add("Content-Type", "application/json")
	_, err := w.Write([]byte("202 - Accepted"))
	if err != nil {
		zl.Warn().Err(err).
			Msg("Can't send status")
	}
}

func (c *Connection) goQueryHandler(w http.ResponseWriter, _ *http.Request) {
	go c.module.Go()
	w.WriteHeader(http.StatusAccepted)
	w.Header().Add("Content-Type", "application/json")
	_, err := w.Write([]byte("202 - Accepted"))
	if err != nil {
		zl.Warn().Err(err).
			Msg("Can't send status")
	}
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
			Int("duration", int(time.Since(start))/int(1e9)).
			Msgf("Called url %s", r.URL.Path)
	})
}

func (c *Connection) Start() {
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

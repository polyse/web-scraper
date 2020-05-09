//+build wireinject

package main

import (
	"os"

	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/api"
	"github.com/polyse/web-scraper/internal/spider"
	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
)

func initApp() (*api.API, func(), error) {
	wire.Build(newConfig, initSpider, initApi)
	return nil, nil, nil
}

func initSpider(cfg *config) (*spider.Spider, error) {
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		zl.Fatal().Err(err).Msgf("Can't parse loglevel")
	}
	zerolog.SetGlobalLevel(logLevel)
	zl.Logger = zl.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	return spider.NewSpider()
}

func initApi(cfg *config, mod *spider.Spider) (*api.API, func(), error) {
	c, err := api.New(cfg.Listen, mod)
	return c, func() {
		c.Close()
	}, err
}

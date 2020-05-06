//+build wireinject

package main

import (
	"os"

	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/app"
	"github.com/polyse/web-scraper/internal/connection"
	"github.com/polyse/web-scraper/internal/module"
	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
)

func initApp() (*app.App, func(), error) {
	wire.Build(newConfig, initModule, initConnection, app.NewApp)
	return nil, nil, nil
}

func initModule(cfg *config) (*module.Module, error) {
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		zl.Fatal().Err(err).Msgf("Can't parse loglevel")
	}
	zerolog.SetGlobalLevel(logLevel)
	zl.Logger = zl.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	return module.NewModule(cfg.OutputPath)
}

func initConnection(cfg *config, mod *module.Module) (*connection.Connection, func(), error) {
	c, err := connection.New(cfg.Listen, mod)
	return c, func() {
		c.Close()
	}, err
}

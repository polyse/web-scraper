//+build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/app"
	"github.com/polyse/web-scraper/internal/connection"
	"github.com/polyse/web-scraper/internal/module"
)

func initApp() (*app.App, func(), error) {
	wire.Build(newConfig, initConnection, initModule, app.NewApp)
	return nil, nil, nil
}

func initModule(cfg *config, conn *connection.Connection) (*module.Module, error) {
	return module.NewModule(cfg.FilePath, cfg.OutputPath, conn)
}

func initConnection(cfg *config) (*connection.Connection, func(), error) {
	c, err := connection.New(cfg.Listen)
	return c, func() {
		c.Close()
	}, err
}

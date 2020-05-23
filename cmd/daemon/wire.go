//+build wireinject

package main

import (
	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/api"
)

func initApp(cfg *config) (*api.API, func(), error) {
	wire.Build(initSpider, initRabbitmq, initApi)
	return nil, nil, nil
}

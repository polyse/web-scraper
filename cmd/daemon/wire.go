//+build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/api"
)

func initApp(ctx context.Context, cfg *config) (*api.API, func(), error) {
	wire.Build(initSpider, initRabbitmq, initApi, initLocker)
	return nil, nil, nil
}

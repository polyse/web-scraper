// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/consumer"
)

func initApp(ctx context.Context, cfg *config) (*consumer.Consumer, func(), error) {
	wire.Build(initConsumer)
	return nil, nil, nil
}

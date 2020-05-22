// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package main

import (
	"context"
	"github.com/polyse/web-scraper/internal/consumer"
)

// Injectors from wire.go:

func initApp(ctx context.Context, cfg *config) (*consumer.Consumer, func(), error) {
	dbClient, err := initSDK(cfg)
	if err != nil {
		return nil, nil, err
	}
	consumerConsumer, cleanup, err := initConsumer(ctx, cfg, dbClient)
	if err != nil {
		return nil, nil, err
	}
	return consumerConsumer, func() {
		cleanup()
	}, nil
}

// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package main

import (
	"context"
	"github.com/polyse/web-scraper/internal/api"
)

// Injectors from wire.go:

func initApp(ctx context.Context, cfg *config) (*api.API, func(), error) {
	queue, cleanup, err := initRabbitmq(cfg)
	if err != nil {
		return nil, nil, err
	}
	conn, cleanup2, err := initLocker(cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	spider, cleanup3, err := initSpider(cfg, queue, conn)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	apiAPI, cleanup4, err := initApi(ctx, cfg, spider)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	return apiAPI, func() {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}

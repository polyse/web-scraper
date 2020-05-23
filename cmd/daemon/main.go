package main

import (
	"context"
	"strings"

	"github.com/polyse/web-scraper/internal/api"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	"github.com/polyse/web-scraper/internal/spider"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xlab/closer"
)

func main() {
	defer closer.Close()

	closer.Bind(func() {
		log.Info().Msg("shutdown")
	})

	cfg, err := initConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Can not init config")
	}

	if err := initLogger(cfg); err != nil {
		log.Fatal().Err(err).Msg("Can not init logger")
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	closer.Bind(cancelCtx)

	api, cleanup, err := initApp(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).
			Msg("Can't init app")
	}
	closer.Bind(cleanup)
	if err := api.Start(); err != nil {
		log.Fatal().Err(err).Msg("Can not start app")
	}
}

func initLogger(cfg *config) error {
	logLevel, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(logLevel)
	return nil
}

func initSpider(cfg *config, queue *rabbitmq.Queue) (*spider.Spider, func(), error) {
	s, err := spider.NewSpider(queue, cfg.RateLimit, cfg.SiteDelay, cfg.SiteRandomDelay)
	if err != nil {
		return nil, nil, err
	}
	return s, s.Close, nil
}

func initApi(ctx context.Context, cfg *config, mod *spider.Spider) (*api.API, func(), error) {
	c, err := api.New(ctx, cfg.Listen, cfg.Auth, mod)
	return c, func() {
		c.Close()
	}, err
}

func initRabbitmq(cfg *config) (*rabbitmq.Queue, func(), error) {
	q, closer, err := rabbitmq.Connect(&rabbitmq.Config{
		Uri:       cfg.RabbitmqUri,
		QueueName: cfg.QueueName,
	})
	return q, func() {
		if err := closer(); err != nil {
			log.Debug().Msgf("Error on close queue: %s", err)
		}
	}, err
}

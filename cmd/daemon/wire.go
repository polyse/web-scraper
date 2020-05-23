//+build wireinject

package main

import (
	"os"

	"github.com/polyse/web-scraper/internal/locker"

	"github.com/polyse/web-scraper/internal/rabbitmq"

	"github.com/google/wire"
	"github.com/polyse/web-scraper/internal/api"
	"github.com/polyse/web-scraper/internal/spider"
	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
)

func initApp() (*api.API, func(), error) {
	wire.Build(newConfig, initSpider, initApi, initRabbitmq, initLocker)
	return nil, nil, nil
}

func initSpider(cfg *config, queue *rabbitmq.Queue, l *locker.Conn) (*spider.Spider, error) {
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		zl.Fatal().Err(err).Msgf("Can't parse loglevel")
	}
	zerolog.SetGlobalLevel(logLevel)
	zl.Logger = zl.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	return spider.NewSpider(queue, l)
}

func initApi(cfg *config, mod *spider.Spider) (*api.API, func(), error) {
	c, err := api.New(cfg.Listen, mod)
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
		err := closer()
		if err != nil {
			zl.Debug().Msgf("Error on close queue: %s", err)
		}
	}, err
}

func initLocker(cfg *config) (*locker.Conn, func(), error) {
	c, closer, err := locker.NewConn(&locker.Config{
		Network: cfg.RedisNetwork,
		Addr:    cfg.RedisAddr,
		Size:    cfg.RedisSoze,
	})
	return c, func() {
		err := closer()
		if err != nil {
			zl.Debug().Msgf("Error on close locker: %s", err)
		}
	}, err
}

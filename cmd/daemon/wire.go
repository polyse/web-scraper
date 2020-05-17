//+build wireinject

package main

import (
	"os"

	"github.com/google/wire"
	database_sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/api"
	"github.com/polyse/web-scraper/internal/broker"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	"github.com/polyse/web-scraper/internal/spider"
	"github.com/rs/zerolog"
	zl "github.com/rs/zerolog/log"
)

func initApp() (*api.API, func(), error) {
	wire.Build(newConfig, initSpider, initRabbitmq, InitBroker, initApi)
	return nil, nil, nil
}

func initSpider(cfg *config, queue *rabbitmq.Queue) (*spider.Spider, error) {
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		zl.Fatal().Err(err).Msgf("Can't parse loglevel")
	}
	zerolog.SetGlobalLevel(logLevel)
	zl.Logger = zl.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	return spider.NewSpider(queue)
}

func initApi(cfg *config, mod *spider.Spider, b *broker.Broker) (*api.API, func(), error) {
	c, err := api.New(cfg.Listen, mod, b)
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

func InitBroker(cfg *config, q *rabbitmq.Queue) *broker.Broker {
	newclient := database_sdk.NewDBClient(cfg.RabbitmqUri)
	return broker.NewBroker(q, cfg.RabbitmqUri, cfg.CollectionName, cfg.QueueName, cfg.NumDocument, cfg.Timeout, newclient)
}

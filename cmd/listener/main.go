package main

import (
	"context"
	"strings"

	database_sdk "github.com/polyse/database-sdk"
	"github.com/polyse/web-scraper/internal/consumer"
	"github.com/polyse/web-scraper/internal/rabbitmq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xlab/closer"
)

func main() {
	defer closer.Close()

	cfg, err := initConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Can not init config")
	}

	if err := initLogger(cfg); err != nil {
		log.Fatal().Err(err).Msg("Can not init logger")
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	closer.Bind(cancelCtx)

	c, cleanup, err := initApp(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Can not init app")
	}
	log.Debug().Msg("Init app")
	closer.Bind(cleanup)
	c.Wg.Add(1)
	if err := c.StartConsume(); err != nil {
		log.Fatal().Err(err).Msg("Can not start app")
	}
	c.Wg.Wait()
}

func initConsumer(ctx context.Context, cfg *config) (*consumer.Consumer, func(), error) {
	q, closer, err := rabbitmq.Connect(&rabbitmq.Config{
		Uri:       cfg.RabbitmqUri,
		QueueName: cfg.QueueName,
	})

	newclient := database_sdk.NewDBClient(cfg.Server)
	c := consumer.NewConsumer(ctx, q, cfg.RabbitmqUri, cfg.CollectionName, cfg.QueueName, cfg.NumDocument, cfg.Timeout, newclient)

	return c, func() {
		if err := closer(); err != nil {
			log.Error().Err(err)
		}
	}, err
}

func initLogger(cfg *config) error {
	logLevel, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(logLevel)
	return nil
}

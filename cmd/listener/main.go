package main

import (
	"context"
	"strings"

	sdk "github.com/polyse/database-sdk"
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

func initSDK(cfg *config) *sdk.DBClient {
	return sdk.NewDBClient(cfg.Server)
}

func initConsumer(ctx context.Context, cfg *config, newC *sdk.DBClient) (*consumer.Consumer, func(), error) {
	q, cl, err := rabbitmq.Connect(&rabbitmq.Config{
		Uri:       cfg.RabbitmqUri,
		QueueName: cfg.QueueName,
	})

	in := consumer.In{
		Url:            cfg.Server,
		NumDoc:         cfg.NumDocument,
		Timeout:        cfg.Timeout,
		CollectionName: cfg.CollectionName,
		QueueName:      cfg.QueueName,
	}

	c := consumer.NewConsumer(ctx, q, in, newC)

	return c, func() {
		if err := cl(); err != nil {
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

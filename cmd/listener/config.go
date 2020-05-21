package main

import (
	"time"

	"github.com/caarlos0/env/v6"
)

type config struct {
	Server         string        `env:"Server" envDefault:"http://localhost:9000"`
	LogLevel       string        `env:"LOG_LEVEL" envDefault:"debug"`
	RabbitmqUri    string        `env:"RABBITMQ_URI" envDefault:"amqp://localhost:5672"`
	QueueName      string        `env:"RABBITMQ_QUEUE" envDefault:"sites-info"`
	NumDocument    int           `env:"NUM_DOC" envDefault:"1000"`
	Timeout        time.Duration `env:"TIMEOUT" envDefault:"10s"`
	CollectionName string        `env:"COLL_NAME" envDefault:"default"`
}

func initConfig() (*config, error) {
	cfg := &config{}

	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

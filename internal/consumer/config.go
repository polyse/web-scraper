package main

import (
	"time"

	"github.com/caarlos0/env/v6"
)

type config struct {
	Listen         string        `env:"LISTEN" envDefault:"localhost:7171"`
	LogLevel       string        `env:"LOG_LEVEL" envDefault:"debug"`
	RabbitmqUri    string        `env:"RABBITMQ_URI" envDefault:"amqp://localhost:5672"`
	QueueName      string        `env:"RABBITMQ_QUEUE" envDefault:"sites-info"`
	NumDocument    int           `env:"NUM_DOC" envDefault:"10000"`
	Timeout        time.Duration `env:"TIMEOUT" envDefault:"100s"`
	CollectionName string        `env:"COLL_NAME" envDefault:"news"`
}

func newConfig() (*config, error) {
	cfg := &config{}

	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

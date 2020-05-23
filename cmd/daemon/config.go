package main

import (
	"time"

	"github.com/caarlos0/env/v6"
)

type config struct {
	Listen          string        `env:"LISTEN" envDefault:"localhost:7171"`
	LogLevel        string        `env:"LOG_LEVEL" envDefault:"debug"`
	RabbitmqUri     string        `env:"RABBITMQ_URI" envDefault:"amqp://localhost:5672"`
	QueueName       string        `env:"RABBITMQ_QUEUE" envDefault:"sites-info"`
	Auth            string        `env:"AUTH_KEY" envDefault:"1234"`
	RateLimit       int           `env:"RATE_LIMIT" envDefault:"10"`
	SiteDelay       time.Duration `env:"SITE_DELAY" envDefault:"1s"`
	SiteRandomDelay time.Duration `env:"SITE_RANDOM_DELAY" envDefault:"3s"`
}

func initConfig() (*config, error) {
	cfg := &config{}

	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

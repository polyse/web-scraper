package main

import (
	"github.com/caarlos0/env/v6"
)

type config struct {
	Listen        string `env:"LISTEN" envDefault:"localhost:7171"`
	LogLevel      string `env:"LOG_LEVEL" envDefault:"debug"`
	RabbitmqUri   string `env:"RABBITMQ_URI" envDefault:"amqp://localhost:5672"`
	QueueName     string `env:"RABBITMQ_QUEUE" envDefault:"sites-info"`
	LockerNetwork string `env:"LOCKER_NETWORK" envDefault:"tcp"`
	LockerAddr    string `env:"LOCKER_ADDR" envDefault:":6379"`
	LockerSize    int    `env:"LOCKER_SIZE" envDefault:"1"`
}

func newConfig() (*config, error) {
	cfg := &config{}

	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

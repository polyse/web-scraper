package main

import (
	"github.com/caarlos0/env/v6"
)

type config struct {
	Listen     string `env:"LISTEN" envDefault:"localhost:7171"`
	LogLevel   string `env:"LOGLEVEL" envDefault:"debug"`
	FilePath   string `env:"FILEPATH" envDefault:"in.txt"`
	OutputPath string `env:"OUTPATH" envDefault:"out.json"`
}

func newConfig() (*config, error) {
	cfg := &config{}

	if err := env.Parse(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

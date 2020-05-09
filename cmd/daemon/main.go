package main

import (
	zl "github.com/rs/zerolog/log"
)

func main() {
	api, cleanup, err := initApp()

	if err != nil {
		zl.Fatal().Err(err).
			Msg("Can't init api")
	}
	defer cleanup()
	api.Start()
}

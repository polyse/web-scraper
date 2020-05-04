package main

import (
	zl "github.com/rs/zerolog/log"
)

func main() {
	app, cleanup, err := initApp()
	if err != nil {
		cleanup()
		zl.Fatal().Err(err).
			Msg("Can't init app")
	}
	defer cleanup()
	app.Action()
}

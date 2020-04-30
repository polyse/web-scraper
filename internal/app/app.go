package app

import (
	"github.com/polyse/web-scraper/internal/module"
)

type App struct {
	mod *module.Module
}

func NewApp(mod *module.Module) (*App, error) {
	return &App{
		mod: mod,
	}, nil
}

func (a *App) Action() {
	a.mod.Conn.Start()
}

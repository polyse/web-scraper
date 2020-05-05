package app

import (
	"github.com/polyse/web-scraper/internal/connection"
)

type App struct {
	con *connection.Connection
}

func NewApp(con *connection.Connection) (*App, error) {
	return &App{
		con: con,
	}, nil
}

func (a *App) Action() {
	a.con.Start()
}

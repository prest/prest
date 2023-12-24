package router

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
)

type Config struct {
	router       *mux.Router
	serverConfig *config.Prest
}

func New(c *config.Prest) *Config {
	cfg := &Config{
		serverConfig: c,
	}
	cfg.Get()
	return cfg
}

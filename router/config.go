package router

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
)

type Config struct {
	router       *mux.Router
	ServerConfig *config.Prest
}

func New(c *config.Prest) *Config {
	cfg := &Config{
		ServerConfig: c,
	}
	cfg.Get()
	return cfg
}

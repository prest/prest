package router

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
)

type Config struct {
	router *mux.Router
	srvCfg *config.Prest
}

func New(c *config.Prest) *Config {
	cfg := &Config{
		srvCfg: c,
	}
	cfg.Get()
	return cfg
}

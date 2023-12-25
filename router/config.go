package router

import (
	"github.com/gorilla/mux"

	"github.com/prest/prest/config"
)

type Config struct {
	router *mux.Router
	srvCfg *config.Prest
}

func New(c *config.Prest) (*Config, error) {
	cfg := &Config{srvCfg: c}
	err := cfg.ConfigRoutes()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

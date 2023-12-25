package router

import (
	"log"

	"github.com/gorilla/mux"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
)

type Config struct {
	router *mux.Router
	srvCfg *config.Prest
}

func New(c *config.Prest) (*Config, error) {
	cfg := &Config{srvCfg: c}
	server, err := controllers.New(c, log.Default())
	if err != nil {
		return nil, err
	}
	err = cfg.ConfigRoutes(server)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

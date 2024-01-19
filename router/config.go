package router

import (
	"log"

	"github.com/gorilla/mux"

	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/plugins"
)

type Config struct {
	router *mux.Router
	srvCfg *config.Prest
	cache  cache.Cacher
}

func New(c *config.Prest, cacher cache.Cacher, pl plugins.Loader) (*Config, error) {
	cfg := &Config{
		srvCfg: c,
		cache:  cacher,
	}
	server, err := controllers.New(c, log.Default(), cacher, pl)
	if err != nil {
		return nil, err
	}
	err = cfg.ConfigRoutes(server)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

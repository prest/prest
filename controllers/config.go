package controllers

import (
	"log"

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/config"
)

type Config struct {
	server  *config.Prest
	adapter adapters.Adapter
	logger  *log.Logger
}

func New(cfg *config.Prest, logger *log.Logger) *Config {
	return &Config{
		server:  cfg,
		adapter: cfg.Adapter,
		logger:  logger,
	}
}

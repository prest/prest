package controllers

import (
	"log"

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/config"
)

// Config
// server holds the configuration for the Prest server.
// adapter is the database adapter used by the Prest server.
// logger is the logger used by the Prest server.
type Config struct {
	server  *config.Prest
	adapter adapters.Adapter
	logger  *log.Logger
}

// New creates a new Config instance with the given configuration and logger.
// It initializes the adapter based on the provided configuration.
// Returns a pointer to the newly created Config instance and an error if any.
func New(cfg *config.Prest, logger *log.Logger) (*Config, error) {
	adptr, err := adapters.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Config{
		server:  cfg,
		adapter: adptr,
		logger:  logger,
	}, nil
}

func (c *Config) GetAdapter() adapters.Adapter {
	return c.adapter
}

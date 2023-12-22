package router

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
)

type Config struct {
	MuxRouter    *mux.Router
	ServerConfig *config.Prest
}

func NewRouter(c *config.Prest) *Config {
	return &Config{
		ServerConfig: c,
	}
}

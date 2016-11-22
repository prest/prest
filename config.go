package config

import (
	"os"

	"github.com/jackc/pgx"
)

// PrestPg PostgreSQL connection config
func PrestPg() pgx.ConnConfig {
	var config pgx.ConnConfig

	config.Host = os.Getenv("PREST_HOST")
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}

	config.User = os.Getenv("PREST_USER")
	if config.User == "" {
		config.User = os.Getenv("USER")
	}

	config.Password = os.Getenv("PREST_PASSWORD")
	config.Database = os.Getenv("PREST_DATABASE")

	return config
}

package controllers

import (
	"log"
	"net/http"

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/config"
	"github.com/prest/prest/plugins"
)

type Server interface {
	GetAdapter() adapters.Adapter

	// auth file
	Auth(w http.ResponseWriter, r *http.Request)

	// databases file
	GetDatabases(w http.ResponseWriter, r *http.Request)

	// healthcheck file
	WrappedHealthCheck(checks CheckList) http.HandlerFunc

	// schemas file
	GetSchemas(w http.ResponseWriter, r *http.Request)

	// sql file
	ExecuteFromScripts(w http.ResponseWriter, r *http.Request)

	// tables file
	GetTables(w http.ResponseWriter, r *http.Request)
	GetTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request)
	SelectFromTables(w http.ResponseWriter, r *http.Request)
	InsertInTables(w http.ResponseWriter, r *http.Request)
	BatchInsertInTables(w http.ResponseWriter, r *http.Request)
	DeleteFromTable(w http.ResponseWriter, r *http.Request)
	UpdateTable(w http.ResponseWriter, r *http.Request)
	ShowTable(w http.ResponseWriter, r *http.Request)
	// v2 auto generated ideas
	// GetColumns(w http.ResponseWriter, r *http.Request)
	// GetFunctions(w http.ResponseWriter, r *http.Request)
	// GetIndexes(w http.ResponseWriter, r *http.Request)
	// GetConstraints(w http.ResponseWriter, r *http.Request)

	// plugins file
	Plugin(w http.ResponseWriter, r *http.Request)
}

// Config
// server holds the configuration for the Prest server.
// adapter is the database adapter used by the Prest server.
// logger is the logger used by the Prest server.
type Config struct {
	server  *config.Prest
	adapter adapters.Adapter
	logger  *log.Logger
	plugins *plugins.Config
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
		plugins: plugins.New(cfg.PluginPath),
	}, nil
}

func (c *Config) GetAdapter() adapters.Adapter {
	return c.adapter
}

package app

import (
	"fmt"

	"github.com/prest/prest/v2/config"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// EnsureAuthTable creates the configured auth users table when missing.
func EnsureAuthTable(cfg *config.Prest, db *sqlx.DB) error {
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (id serial PRIMARY KEY, name text, username text unique, password text, metadata jsonb)",
		pq.QuoteIdentifier(cfg.AuthSchema),
		pq.QuoteIdentifier(cfg.AuthTable),
	))
	return err
}

// EnsureQueriesTable creates the configured prest_queries table and location index when missing.
func EnsureQueriesTable(cfg *config.Prest, db *sqlx.DB) error {
	schema := pq.QuoteIdentifier(cfg.QueriesConf.Schema)
	table := pq.QuoteIdentifier(cfg.QueriesConf.Table)
	index := pq.QuoteIdentifier(fmt.Sprintf("%s_location_idx", cfg.QueriesConf.Table))

	_, err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
  id             BIGSERIAL PRIMARY KEY,
  database_alias TEXT NOT NULL DEFAULT '',
  location       TEXT NOT NULL,
  name           TEXT NOT NULL,
  read_sql       TEXT,
  write_sql      TEXT,
  update_sql     TEXT,
  delete_sql     TEXT,
  description    TEXT,
  created_by     TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (database_alias, location, name)
)`, schema, table))
	if err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s.%s (location)",
		index, schema, table,
	))
	return err
}

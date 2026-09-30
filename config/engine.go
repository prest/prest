package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	// EnginePostgres is the default database engine.
	EnginePostgres = "postgres"
	// EngineMySQL selects the MySQL dialect adapter.
	EngineMySQL = "mysql"

	mysqlDefaultPort = 3306
)

// ErrUnknownEngine is returned when engine is not postgres or mysql.
var ErrUnknownEngine = fmt.Errorf("unknown database engine")

// ErrMySQLConfig is returned when engine=mysql is missing required settings
// or asks for client certificates this version does not configure.
var ErrMySQLConfig = fmt.Errorf("invalid mysql configuration")

// EffectiveEngine resolves a registry entry's engine.
// An empty entry engine inherits the root engine. An empty root engine is postgres.
func EffectiveEngine(db *DatabaseConf, cfg *Prest) string {
	if db != nil {
		if e := normalizeEngine(db.Engine); e != "" {
			return e
		}
	}
	if cfg != nil {
		if e := normalizeEngine(cfg.Engine); e != "" {
			return e
		}
	}
	return EnginePostgres
}

func normalizeEngine(engine string) string {
	return strings.ToLower(strings.TrimSpace(engine))
}

// ValidateEngines fails closed for an unknown engine and for mysql entries
// that omit user or database, or that set client certificate paths.
func (p *Prest) ValidateEngines() error {
	if p == nil {
		return nil
	}
	root := EffectiveEngine(nil, p)
	if err := checkEngineName(root); err != nil {
		return err
	}
	if root == EngineMySQL {
		if err := checkMySQLFields("mysql", p.PGUser, p.PGDatabase, p.PGSSLCert, p.PGSSLKey, p.PGSSLRootCert); err != nil {
			return err
		}
	}
	for i := range p.Databases {
		db := &p.Databases[i]
		eng := EffectiveEngine(db, p)
		if err := checkEngineName(eng); err != nil {
			return fmt.Errorf("database %s: %w", db.Alias, err)
		}
		if eng == EngineMySQL {
			if err := checkMySQLFields(db.Alias, db.User, db.Database, db.SSL.Cert, db.SSL.Key, db.SSL.RootCert); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkEngineName(engine string) error {
	switch engine {
	case EnginePostgres, EngineMySQL:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownEngine, engine)
	}
}

func checkMySQLFields(alias, user, database, cert, key, rootCert string) error {
	if user == "" || database == "" {
		return fmt.Errorf("%w: %s requires user and database", ErrMySQLConfig, alias)
	}
	if cert != "" || key != "" || rootCert != "" {
		return fmt.Errorf("%w: %s client certificates are not supported", ErrMySQLConfig, alias)
	}
	return nil
}

func urlScheme(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Scheme)
}

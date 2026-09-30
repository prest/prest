package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/spf13/viper"
)

// DatabaseSSLConf holds per-database SSL settings from TOML.
type DatabaseSSLConf struct {
	Mode     string `mapstructure:"mode"`
	Cert     string `mapstructure:"cert"`
	Key      string `mapstructure:"key"`
	RootCert string `mapstructure:"rootcert"`
}

// DatabaseConf describes a registered database alias and its connection profile.
type DatabaseConf struct {
	Alias       string          `mapstructure:"alias"`
	Engine      string          `mapstructure:"engine"`
	URL         string          `mapstructure:"url"`
	Host        string          `mapstructure:"host"`
	Port        int             `mapstructure:"port"`
	User        string          `mapstructure:"user"`
	Pass        string          `mapstructure:"pass"`
	Database    string          `mapstructure:"database"`
	SSL         DatabaseSSLConf `mapstructure:"ssl"`
	MaxOpenConn int             `mapstructure:"maxopenconn"`
	MaxIdleConn int             `mapstructure:"maxidleconn"`
}

// HasDatabaseRegistry reports whether a multi-database registry is configured.
func (p *Prest) HasDatabaseRegistry() bool {
	return p != nil && len(p.Databases) > 0
}

// parseDatabaseRegistry parses the database registry from the environment and
// the configuration file. It merges the entries and fills in defaults.
func parseDatabaseRegistry(v *viper.Viper, cfg *Prest) {
	merged := make(map[string]DatabaseConf)

	for _, db := range parseDatabaseRegistryFromEnv() {
		addDatabaseConf(merged, db)
	}
	envAliases := make(map[string]struct{}, len(merged))
	for alias := range merged {
		envAliases[alias] = struct{}{}
	}

	var tomlDBs []DatabaseConf
	if raw := v.Get("databases"); raw != nil {
		if _, isString := raw.(string); !isString {
			if err := v.UnmarshalKey("databases", &tomlDBs); err != nil {
				slog.Warn("config key invalid, using default", "key", "databases", "err", err)
				tomlDBs = nil
			}
		}
	}
	for _, db := range tomlDBs {
		if db.Alias == "" {
			slog.Warn("database registry entry skipped: missing alias")
			continue
		}
		if !ident.IsSafeSegment(db.Alias) {
			slog.Warn("database registry entry skipped: invalid alias", "alias", db.Alias)
			continue
		}
		if db.Engine == "" && urlScheme(db.URL) == EngineMySQL {
			db.Engine = EngineMySQL
		}
		fillDatabaseDefaults(&db, cfg)
		if _, ok := envAliases[db.Alias]; ok {
			continue
		}
		addDatabaseConf(merged, db)
	}

	if len(merged) == 0 {
		cfg.Databases = nil
		return
	}

	cfg.Databases = sortedDatabaseConfs(merged)
}

func addDatabaseConf(merged map[string]DatabaseConf, db DatabaseConf) {
	if db.Alias == "" {
		slog.Warn("database registry entry skipped: missing alias")
		return
	}
	if !ident.IsSafeSegment(db.Alias) {
		slog.Warn("database registry entry skipped: invalid alias", "alias", db.Alias)
		return
	}
	if _, exists := merged[db.Alias]; exists {
		slog.Warn("database registry entry skipped: duplicate alias", "alias", db.Alias)
		return
	}
	if db.URL != "" {
		applyURLToDatabaseConf(&db)
	}
	merged[db.Alias] = db
}

func parseDatabaseRegistryFromEnv() []DatabaseConf {
	var dbs []DatabaseConf
	for i := 1; ; i++ {
		alias := envFirst(
			fmt.Sprintf("DATABASE_ALIAS_%d", i),
			fmt.Sprintf("PREST_DATABASE_ALIAS_%d", i),
		)
		connURL := envFirst(
			fmt.Sprintf("DATABASE_URL_%d", i),
			fmt.Sprintf("PREST_DATABASE_URL_%d", i),
		)
		if alias == "" && connURL == "" {
			break
		}
		if alias == "" {
			slog.Warn(
				"database registry entry skipped: URL without alias",
				"index", i,
				"env", fmt.Sprintf("DATABASE_URL_%d", i),
			)
			continue
		}
		if connURL == "" {
			slog.Warn(
				"database registry entry skipped: alias without URL",
				"alias", alias,
				"index", i,
				"env", fmt.Sprintf("DATABASE_ALIAS_%d", i),
			)
			continue
		}
		if !ident.IsSafeSegment(alias) {
			slog.Warn("database registry entry skipped: invalid alias", "alias", alias, "index", i)
			continue
		}
		conf := DatabaseConf{Alias: alias, URL: connURL}
		applyURLToDatabaseConf(&conf)
		dbs = append(dbs, conf)
	}
	return dbs
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func applyURLToDatabaseConf(db *DatabaseConf) {
	if db.URL == "" {
		return
	}
	u, err := url.Parse(db.URL)
	if err != nil {
		// url.Parse embeds the full original URL (credentials included) in its
		// own error message, so err must be redacted too, not just the url field.
		slog.Warn(
			"database URL invalid, using defaults for connection fields",
			"alias", db.Alias,
			"url", logsafe.Redact(db.URL),
			"err", logsafe.Error(err),
		)
		return
	}
	if strings.EqualFold(u.Scheme, EngineMySQL) {
		if db.Engine != "" && normalizeEngine(db.Engine) != EngineMySQL {
			slog.Warn(
				"mysql URL ignored: engine is not mysql",
				"alias", db.Alias,
				"url", logsafe.Redact(db.URL),
			)
			return
		}
		db.Engine = EngineMySQL
		applyMySQLURL(db, u)
		return
	}
	if EffectiveEngine(db, nil) == EngineMySQL {
		slog.Warn(
			"mysql database URL ignored: scheme is not mysql",
			"alias", db.Alias,
			"url", logsafe.Redact(db.URL),
		)
		return
	}
	if u.Hostname() != "" {
		db.Host = u.Hostname()
	}
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			db.Port = port
		}
	}
	if u.User != nil {
		db.User = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			db.Pass = pass
		}
	}
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		db.Database = path
	}
	if mode := u.Query().Get("sslmode"); mode != "" {
		db.SSL.Mode = mode
	}
}

func applyMySQLURL(db *DatabaseConf, u *url.URL) {
	if u.Hostname() != "" {
		db.Host = u.Hostname()
	}
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			db.Port = port
		}
	}
	if db.Port == 0 {
		db.Port = mysqlDefaultPort
	}
	if u.User != nil {
		if user := u.User.Username(); user != "" {
			db.User = user
		}
		if pass, ok := u.User.Password(); ok {
			db.Pass = pass
		}
	}
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		db.Database = path
	}
	if mode := mysqlTLSModeFromQuery(u); mode != "" {
		db.SSL.Mode = mode
	}
	if db.SSL.Mode == "" {
		db.SSL.Mode = "disable"
	}
}

func fillDatabaseDefaults(db *DatabaseConf, cfg *Prest) {
	if EffectiveEngine(db, cfg) == EngineMySQL {
		fillMySQLDatabaseDefaults(db, cfg)
		return
	}
	if db.Host == "" {
		db.Host = cfg.PGHost
	}
	if db.Port == 0 {
		db.Port = cfg.PGPort
	}
	if db.User == "" {
		db.User = cfg.PGUser
	}
	if db.Pass == "" {
		db.Pass = cfg.PGPass
	}
	if db.Database == "" {
		db.Database = cfg.PGDatabase
	}
	if db.SSL.Mode == "" {
		db.SSL.Mode = cfg.PGSSLMode
	}
	if db.SSL.Cert == "" {
		db.SSL.Cert = cfg.PGSSLCert
	}
	if db.SSL.Key == "" {
		db.SSL.Key = cfg.PGSSLKey
	}
	if db.SSL.RootCert == "" {
		db.SSL.RootCert = cfg.PGSSLRootCert
	}
	if db.MaxOpenConn == 0 {
		db.MaxOpenConn = cfg.PGMaxOpenConn
	}
	if db.MaxIdleConn == 0 {
		db.MaxIdleConn = cfg.PGMaxIdleConn
	}
}

// fillMySQLDatabaseDefaults does not copy Postgres host, user, password, or
// database. Port 3306 applies only when the entry port is unset. When the
// root engine is also mysql, unset fields inherit the root mysql profile.
func fillMySQLDatabaseDefaults(db *DatabaseConf, cfg *Prest) {
	rootMySQL := cfg != nil && EffectiveEngine(nil, cfg) == EngineMySQL
	if db.Host == "" && rootMySQL {
		db.Host = cfg.PGHost
	}
	if db.Port == 0 {
		if rootMySQL && cfg.PGPort != 0 {
			db.Port = cfg.PGPort
		} else {
			db.Port = mysqlDefaultPort
		}
	}
	if db.User == "" && rootMySQL {
		db.User = cfg.PGUser
	}
	if db.Pass == "" && rootMySQL {
		db.Pass = cfg.PGPass
	}
	if db.Database == "" && rootMySQL {
		db.Database = cfg.PGDatabase
	}
	if db.SSL.Mode == "" {
		if rootMySQL && cfg.PGSSLMode != "" {
			db.SSL.Mode = cfg.PGSSLMode
		} else {
			db.SSL.Mode = "disable"
		}
	}
	if rootMySQL {
		if db.SSL.Cert == "" {
			db.SSL.Cert = cfg.PGSSLCert
		}
		if db.SSL.Key == "" {
			db.SSL.Key = cfg.PGSSLKey
		}
		if db.SSL.RootCert == "" {
			db.SSL.RootCert = cfg.PGSSLRootCert
		}
	}
	if db.MaxOpenConn == 0 && cfg != nil {
		db.MaxOpenConn = cfg.PGMaxOpenConn
	}
	if db.MaxIdleConn == 0 && cfg != nil {
		db.MaxIdleConn = cfg.PGMaxIdleConn
	}
}

func sortedDatabaseConfs(merged map[string]DatabaseConf) []DatabaseConf {
	aliases := make([]string, 0, len(merged))
	for alias := range merged {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	out := make([]DatabaseConf, 0, len(aliases))
	for _, alias := range aliases {
		out = append(out, merged[alias])
	}
	return out
}

// ProfileByAlias returns the connection profile for alias when a registry is configured.
func (p *Prest) ProfileByAlias(alias string) (DatabaseConf, bool) {
	if !p.HasDatabaseRegistry() {
		return DatabaseConf{}, false
	}
	for _, db := range p.Databases {
		if db.Alias == alias {
			return db, true
		}
	}
	return DatabaseConf{}, false
}

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
func HasDatabaseRegistry(cfg *Prest) bool {
	return cfg != nil && len(cfg.Databases) > 0
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
		slog.Warn(
			"database URL invalid, using defaults for connection fields",
			"alias", db.Alias,
			"url", db.URL,
			"err", err,
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

func fillDatabaseDefaults(db *DatabaseConf, cfg *Prest) {
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
func ProfileByAlias(cfg *Prest, alias string) (DatabaseConf, bool) {
	if !HasDatabaseRegistry(cfg) {
		return DatabaseConf{}, false
	}
	for _, db := range cfg.Databases {
		if db.Alias == alias {
			return db, true
		}
	}
	return DatabaseConf{}, false
}

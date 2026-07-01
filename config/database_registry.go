package config

import (
	"fmt"
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

func parseDatabaseRegistry(cfg *Prest) error {
	merged := make(map[string]DatabaseConf)

	indexed, err := parseDatabaseRegistryFromEnv()
	if err != nil {
		return err
	}
	for _, db := range indexed {
		if err := addDatabaseConf(merged, db); err != nil {
			return err
		}
	}

	manifest, err := parseDatabaseRegistryFromManifestEnv()
	if err != nil {
		return err
	}
	for _, db := range manifest {
		if existing, ok := merged[db.Alias]; ok {
			if db.URL != "" {
				existing.URL = db.URL
				applyURLToDatabaseConf(&existing)
				merged[db.Alias] = existing
			}
			continue
		}
		if err := addDatabaseConf(merged, db); err != nil {
			return err
		}
	}

	var tomlDBs []DatabaseConf
	if raw := viper.Get("databases"); raw != nil {
		if _, isString := raw.(string); !isString {
			if err := viper.UnmarshalKey("databases", &tomlDBs); err != nil {
				return fmt.Errorf("unmarshal databases: %w", err)
			}
		}
	}
	for _, db := range tomlDBs {
		if db.Alias == "" {
			return fmt.Errorf("database entry missing alias")
		}
		if !ident.IsSafeSegment(db.Alias) {
			return fmt.Errorf("invalid database alias %q", db.Alias)
		}
		fillDatabaseDefaults(&db, cfg)
		if _, ok := merged[db.Alias]; !ok {
			if err := addDatabaseConf(merged, db); err != nil {
				return err
			}
		}
	}

	// Manifest aliases without URL can be completed from TOML.
	for alias, db := range merged {
		if db.URL != "" || hasConnectionFields(db) {
			continue
		}
		for _, t := range tomlDBs {
			if t.Alias == alias {
				merged[alias] = mergeDatabaseConf(t, db)
				break
			}
		}
		if merged[alias].URL == "" && !hasConnectionFields(merged[alias]) {
			return fmt.Errorf("database %q has no connection URL or host settings", alias)
		}
	}

	if len(merged) == 0 {
		cfg.Databases = nil
		return nil
	}

	cfg.Databases = sortedDatabaseConfs(merged)
	return nil
}

func addDatabaseConf(merged map[string]DatabaseConf, db DatabaseConf) error {
	if db.Alias == "" {
		return fmt.Errorf("database entry missing alias")
	}
	if !ident.IsSafeSegment(db.Alias) {
		return fmt.Errorf("invalid database alias %q", db.Alias)
	}
	if _, exists := merged[db.Alias]; exists {
		return fmt.Errorf("duplicate database alias %q", db.Alias)
	}
	if db.URL != "" {
		applyURLToDatabaseConf(&db)
	}
	merged[db.Alias] = db
	return nil
}

func parseDatabaseRegistryFromEnv() ([]DatabaseConf, error) {
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
			return nil, fmt.Errorf("DATABASE_URL_%d set without DATABASE_ALIAS_%d", i, i)
		}
		if connURL == "" {
			return nil, fmt.Errorf("DATABASE_ALIAS_%d set without DATABASE_URL_%d", i, i)
		}
		if !ident.IsSafeSegment(alias) {
			return nil, fmt.Errorf("invalid database alias %q at index %d", alias, i)
		}
		conf := DatabaseConf{Alias: alias, URL: connURL}
		applyURLToDatabaseConf(&conf)
		dbs = append(dbs, conf)
	}
	return dbs, nil
}

func parseDatabaseRegistryFromManifestEnv() ([]DatabaseConf, error) {
	manifest := strings.TrimSpace(os.Getenv("PREST_DATABASES"))
	if manifest == "" {
		return nil, nil
	}

	var dbs []DatabaseConf
	for _, raw := range strings.Split(manifest, ",") {
		alias := strings.TrimSpace(raw)
		if alias == "" {
			continue
		}
		if !ident.IsSafeSegment(alias) {
			return nil, fmt.Errorf("invalid database alias %q in PREST_DATABASES", alias)
		}
		envKey := "PREST_DATABASE_" + manifestAliasEnvKey(alias) + "_URL"
		connURL := os.Getenv(envKey)
		conf := DatabaseConf{Alias: alias, URL: connURL}
		if connURL != "" {
			applyURLToDatabaseConf(&conf)
		}
		dbs = append(dbs, conf)
	}
	return dbs, nil
}

func manifestAliasEnvKey(alias string) string {
	return strings.ToUpper(strings.ReplaceAll(alias, "-", "_"))
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

func hasConnectionFields(db DatabaseConf) bool {
	return db.Host != "" || db.URL != ""
}

func mergeDatabaseConf(toml, existing DatabaseConf) DatabaseConf {
	out := toml
	out.Alias = existing.Alias
	if existing.URL != "" {
		out.URL = existing.URL
		applyURLToDatabaseConf(&out)
	}
	return out
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

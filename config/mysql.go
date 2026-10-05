package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/spf13/viper"
)

func setMySQLDefaults(v *viper.Viper) {
	v.SetDefault("mysql.prepare", false)
}

// parseMySQLDBConfig reads connection fields without applying Postgres defaults.
// viper defaults for pg.user, pg.pass, pg.database, and pg.host are ignored
// unless the key was set in the file or environment. Port 3306 applies only
// when port is unset. A mysql:// URL is applied; any other scheme is ignored.
func parseMySQLDBConfig(v *viper.Viper, cfg *Prest) error {
	cfg.MySQLPrepare = v.GetBool("mysql.prepare")
	if host, ok := mysqlExplicitString(v, "pg.host"); ok {
		cfg.PGHost = host
	}
	port, portSet, err := mysqlExplicitPort(v)
	if err != nil {
		return err
	}
	if portSet {
		cfg.PGPort = port
	}
	if cfg.PGPort == 0 {
		cfg.PGPort = mysqlDefaultPort
	}
	if user, ok := mysqlExplicitString(v, "pg.user"); ok {
		cfg.PGUser = user
	}
	if pass, ok := mysqlExplicitString(v, "pg.pass"); ok {
		cfg.PGPass = pass
	}
	if database, ok := mysqlExplicitString(v, "pg.database"); ok {
		cfg.PGDatabase = database
	}
	if mode, ok := mysqlExplicitString(v, "pg.ssl.mode"); ok {
		cfg.PGSSLMode = mode
	}
	if cfg.PGSSLMode == "" {
		cfg.PGSSLMode = "disable"
	}
	if key, ok := mysqlExplicitString(v, "pg.ssl.key"); ok {
		cfg.PGSSLKey = key
	}
	if cert, ok := mysqlExplicitString(v, "pg.ssl.cert"); ok {
		cfg.PGSSLCert = cert
	}
	if root, ok := mysqlExplicitString(v, "pg.ssl.rootcert"); ok {
		cfg.PGSSLRootCert = root
	}

	cfg.PGMaxIdleConn, err = configuredInt(v, "pg.maxidleconn")
	if err != nil {
		return err
	}
	cfg.PGMaxOpenConn, err = configuredInt(v, "pg.maxopenconn")
	if err != nil {
		return err
	}
	cfg.PGConnTimeout, err = configuredInt(v, "pg.conntimeout")
	if err != nil {
		return err
	}
	cfg.PGCache, err = configuredBool(v, "pg.cache")
	if err != nil {
		return err
	}
	cfg.SingleDB, err = configuredBool(v, "pg.single")
	if err != nil {
		return err
	}

	if rawURL, ok := mysqlExplicitString(v, "pg.url"); ok {
		cfg.PGURL = rawURL
	}
	if os.Getenv("DATABASE_URL") != "" {
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	return applyMySQLURLToPrest(cfg)
}

// configEnvNames maps a viper key to the engine-neutral name and the legacy name.
// pg.host becomes PREST_DB_HOST and PREST_PG_HOST. auth.schema becomes
// PREST_DB_AUTH_SCHEMA and PREST_AUTH_SCHEMA. The legacy name keeps the PREST_
// prefix viper already uses, so PREST_PG_* and PREST_AUTH_SCHEMA stay valid.
func configEnvNames(key string) (dbEnv, legacyEnv string) {
	upper := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	suffix := strings.TrimPrefix(upper, "PG_")
	return "PREST_DB_" + suffix, "PREST_" + upper
}

// lookupConfigEnv reads PREST_DB_* first, then the legacy PREST_* name.
// An explicit empty value is present.
func lookupConfigEnv(key string) (string, bool) {
	dbEnv, legacyEnv := configEnvNames(key)
	if val, ok := os.LookupEnv(dbEnv); ok {
		return val, true
	}
	if val, ok := os.LookupEnv(legacyEnv); ok {
		return val, true
	}
	return "", false
}

// mysqlExplicitString returns a string from the environment or the config file.
// Viper defaults are not treated as set. GetString is used only for a file
// value, after the environment is known to be unset.
func mysqlExplicitString(v *viper.Viper, key string) (string, bool) {
	if val, ok := lookupConfigEnv(key); ok {
		return val, true
	}
	if v != nil && v.InConfig(key) {
		return v.GetString(key), true
	}
	return "", false
}

// mysqlConfigured reports a value from the environment or config file.
// viper defaults for the Postgres keys are not treated as set.
func mysqlConfigured(v *viper.Viper, key string) bool {
	_, ok := mysqlExplicitString(v, key)
	return ok
}

// mysqlExplicitPort parses PREST_DB_PORT or PREST_PG_PORT with Atoi when that
// variable is present. A file value still uses GetInt. Unset stays 0 so the
// caller can apply 3306. A non-numeric env value fails.
func mysqlExplicitPort(v *viper.Viper) (port int, ok bool, err error) {
	if val, found := lookupConfigEnv("pg.port"); found {
		port, err = strconv.Atoi(val)
		if err != nil {
			return 0, false, fmt.Errorf("invalid pg.port %q: %w", val, err)
		}
		return port, true, nil
	}
	if mysqlConfigured(v, "pg.port") {
		return v.GetInt("pg.port"), true, nil
	}
	return 0, false, nil
}

// configuredString uses an explicit env value, including empty, and otherwise
// viper (file or Postgres default).
func configuredString(v *viper.Viper, key string) string {
	if val, ok := lookupConfigEnv(key); ok {
		return val
	}
	return v.GetString(key)
}

// configuredInt parses PREST_DB_* / PREST_PG_* with Atoi when present.
// Otherwise it uses viper, including Postgres defaults.
func configuredInt(v *viper.Viper, key string) (int, error) {
	if val, ok := lookupConfigEnv(key); ok {
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q: %w", key, val, err)
		}
		return n, nil
	}
	return v.GetInt(key), nil
}

// configuredBool parses PREST_DB_* / PREST_PG_* when present.
// An explicit empty value is false. Otherwise it uses viper.
func configuredBool(v *viper.Viper, key string) (bool, error) {
	if val, ok := lookupConfigEnv(key); ok {
		if val == "" {
			return false, nil
		}
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, fmt.Errorf("invalid %s %q: %w", key, val, err)
		}
		return b, nil
	}
	return v.GetBool(key), nil
}

func applyMySQLURLToPrest(cfg *Prest) error {
	if cfg.PGURL == "" {
		return nil
	}
	u, err := url.Parse(cfg.PGURL)
	if err != nil {
		slog.Error("cannot parse mysql url", "err", logsafe.Error(err))
		// url.Error.Error() embeds the raw URL, including userinfo.
		return fmt.Errorf("cannot parse mysql url: %s", mysqlURLParseCause(err))
	}
	if !strings.EqualFold(u.Scheme, "mysql") {
		slog.Error("mysql URL ignored: scheme is not mysql", "scheme", u.Scheme)
		cfg.PGURL = ""
		return nil
	}
	port := 0
	hasPort := false
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil {
			return fmt.Errorf("cannot parse mysql url port %q: %w", u.Port(), err)
		}
		hasPort = true
	}
	if u.Hostname() != "" {
		cfg.PGHost = u.Hostname()
	}
	if hasPort {
		cfg.PGPort = port
	}
	if u.User != nil {
		if user := u.User.Username(); user != "" {
			cfg.PGUser = user
		}
		if pass, ok := u.User.Password(); ok {
			cfg.PGPass = pass
		}
	}
	if path := strings.TrimPrefix(u.Path, "/"); path != "" {
		cfg.PGDatabase = path
	}
	if mode := mysqlTLSModeFromQuery(u); mode != "" {
		cfg.PGSSLMode = mode
	}
	return nil
}

// mysqlURLParseCause is the reason from url.Parse without the raw URL.
// Wrapping the *url.Error itself would put userinfo in Error().
func mysqlURLParseCause(err error) string {
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		return uerr.Err.Error()
	}
	return "invalid url"
}

func mysqlTLSModeFromQuery(u *url.URL) string {
	if u == nil {
		return ""
	}
	if tlsMode := u.Query().Get("tls"); tlsMode != "" {
		switch strings.ToLower(tlsMode) {
		case "false":
			return "disable"
		case "true":
			return "require"
		case "skip-verify":
			return "skip-verify"
		default:
			return tlsMode
		}
	}
	return u.Query().Get("sslmode")
}

// ApplyMySQLRegistrySchemas sets auth and queries schemas to database when
// the operator did not set those keys. A nil config or an empty database is
// left unchanged. Explicit values, including "public", are kept.
func ApplyMySQLRegistrySchemas(dst *Prest, database string) {
	if dst == nil || database == "" {
		return
	}
	if !dst.authSchemaSet {
		dst.AuthSchema = database
	}
	if !dst.queriesSchemaSet {
		dst.QueriesConf.Schema = database
	}
}

// applyMySQLSchemaDefaults copies the MySQL database name into auth and
// queries schemas when those keys were not set in the file or environment.
// viper IsSet is true for SetDefault, so a default of "public" is not explicit.
func applyMySQLSchemaDefaults(v *viper.Viper, cfg *Prest) {
	if cfg == nil || v == nil || EffectiveEngine(nil, cfg) != EngineMySQL || cfg.PGDatabase == "" {
		return
	}
	if !mysqlConfigured(v, "auth.schema") {
		cfg.AuthSchema = cfg.PGDatabase
	}
	if !mysqlConfigured(v, "queries.schema") {
		cfg.QueriesConf.Schema = cfg.PGDatabase
	}
}

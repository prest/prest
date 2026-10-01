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
	if mysqlConfigured(v, "pg.host") {
		cfg.PGHost = v.GetString("pg.host")
	}
	if mysqlConfigured(v, "pg.port") {
		cfg.PGPort = v.GetInt("pg.port")
	}
	if cfg.PGPort == 0 {
		cfg.PGPort = mysqlDefaultPort
	}
	if mysqlConfigured(v, "pg.user") {
		cfg.PGUser = v.GetString("pg.user")
	}
	if mysqlConfigured(v, "pg.pass") {
		cfg.PGPass = v.GetString("pg.pass")
	}
	if mysqlConfigured(v, "pg.database") {
		cfg.PGDatabase = v.GetString("pg.database")
	}
	if mysqlConfigured(v, "pg.ssl.mode") {
		cfg.PGSSLMode = v.GetString("pg.ssl.mode")
	}
	if cfg.PGSSLMode == "" {
		cfg.PGSSLMode = "disable"
	}
	if mysqlConfigured(v, "pg.ssl.key") {
		cfg.PGSSLKey = v.GetString("pg.ssl.key")
	}
	if mysqlConfigured(v, "pg.ssl.cert") {
		cfg.PGSSLCert = v.GetString("pg.ssl.cert")
	}
	if mysqlConfigured(v, "pg.ssl.rootcert") {
		cfg.PGSSLRootCert = v.GetString("pg.ssl.rootcert")
	}

	cfg.PGMaxIdleConn = v.GetInt("pg.maxidleconn")
	cfg.PGMaxOpenConn = v.GetInt("pg.maxopenconn")
	cfg.PGConnTimeout = v.GetInt("pg.conntimeout")
	cfg.PGCache = v.GetBool("pg.cache")
	cfg.SingleDB = v.GetBool("pg.single")

	if mysqlConfigured(v, "pg.url") {
		cfg.PGURL = v.GetString("pg.url")
	}
	if os.Getenv("DATABASE_URL") != "" {
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	return applyMySQLURLToPrest(cfg)
}

// mysqlConfigured reports a value from the environment or config file.
// viper defaults for the Postgres keys are not treated as set.
func mysqlConfigured(v *viper.Viper, key string) bool {
	envKey := "PREST_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	if _, ok := os.LookupEnv(envKey); ok {
		return true
	}
	return v.InConfig(key)
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

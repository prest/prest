package config

import (
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/spf13/viper"
)

func setPostgresDefaults(v *viper.Viper) {
	v.SetDefault("pg.host", "127.0.0.1")
	v.SetDefault("pg.port", 5432)
	v.SetDefault("pg.database", "prest")
	v.SetDefault("pg.user", "postgres")
	v.SetDefault("pg.pass", "postgres")
	v.SetDefault("pg.maxidleconn", 0) // avoids db memory leak on req timeout
	v.SetDefault("pg.maxopenconn", 10)
	v.SetDefault("pg.conntimeout", 10)
	v.SetDefault("pg.cache", true)
	// todo: replace this with prefer, will need to replace lib/pq
	// https://github.com/jackc/pgx/blob/47d631e34be7128997a0aa89b75885cc4ad4c82e/pgconn/config.go#L218
	v.SetDefault("pg.ssl.mode", "disable")
}

// warnDeprecatedSingle logs when the removed pg.single key is still set in the
// config file or environment; the value is ignored.
func warnDeprecatedSingle(v *viper.Viper) {
	_, fromEnv := lookupConfigEnv("pg.single")
	if fromEnv || v.IsSet("pg.single") {
		slog.Warn("config key ignored: pg.single was removed; use [[databases]] or pg.allowed_databases",
			"key", "pg.single")
	}
}

func parseDBConfig(v *viper.Viper, cfg *Prest) error {
	cfg.Engine = normalizeEngine(v.GetString("engine"))
	if cfg.Engine == "" {
		cfg.Engine = EnginePostgres
	}
	warnDeprecatedSingle(v)
	cfg.AllowedDatabases = configuredStringList(v, "pg.allowed_databases")
	if cfg.Engine == EngineMySQL {
		return parseMySQLDBConfig(v, cfg)
	}
	cfg.PGURL = configuredString(v, "pg.url")
	cfg.PGHost = configuredString(v, "pg.host")
	port, err := configuredInt(v, "pg.port")
	if err != nil {
		return err
	}
	cfg.PGPort = port
	cfg.PGUser = configuredString(v, "pg.user")
	cfg.PGPass = configuredString(v, "pg.pass")
	cfg.PGDatabase = configuredString(v, "pg.database")
	cfg.PGSSLMode = configuredString(v, "pg.ssl.mode")
	cfg.PGSSLKey = configuredString(v, "pg.ssl.key")
	cfg.PGSSLCert = configuredString(v, "pg.ssl.cert")
	cfg.PGSSLRootCert = configuredString(v, "pg.ssl.rootcert")

	if os.Getenv("DATABASE_URL") != "" {
		// cloud factor support: https://devcenter.heroku.com/changelog-items/438
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	parseDatabaseURL(cfg)

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
	return nil
}

// configuredStringList reads a list from the env (comma separated) or the
// config file (array or comma-separated string). Items are trimmed and empty
// items dropped.
func configuredStringList(v *viper.Viper, key string) []string {
	var raw []string
	if val, ok := lookupConfigEnv(key); ok {
		raw = strings.Split(val, ",")
	} else if s, ok := v.Get(key).(string); ok {
		raw = strings.Split(s, ",")
	} else {
		raw = v.GetStringSlice(key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseDatabaseURL tries to get from URL the DB configs
func parseDatabaseURL(cfg *Prest) {
	if cfg.PGURL == "" {
		slog.Debug("no db url found, skipping")
		return
	}
	// Parser PG URL, get database connection via string URL
	u, err := url.Parse(cfg.PGURL)
	if err != nil {
		slog.Error("cannot parse db url", "err", logsafe.Error(err))
		return
	}
	// mysql:// is parsed only when engine=mysql, and never by this parser.
	if strings.EqualFold(u.Scheme, "mysql") {
		slog.Error("mysql URL ignored: engine is not mysql")
		return
	}
	cfg.PGHost = u.Hostname()
	if u.Port() != "" {
		pgPort, err := strconv.Atoi(u.Port())
		if err != nil {
			slog.Error("cannot parse db url port, falling back to default values", "port", u.Port(), "err", err)
			return
		}
		cfg.PGPort = pgPort
	}
	cfg.PGUser = u.User.Username()
	pgPass, pgPassExist := u.User.Password()
	if pgPassExist {
		cfg.PGPass = pgPass
	}
	cfg.PGDatabase = strings.Replace(u.Path, "/", "", -1)
	if u.Query().Get("sslmode") != "" {
		cfg.PGSSLMode = u.Query().Get("sslmode")
	}
}

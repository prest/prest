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
	v.SetDefault("pg.single", true)
	v.SetDefault("pg.cache", true)
	// todo: replace this with prefer, will need to replace lib/pq
	// https://github.com/jackc/pgx/blob/47d631e34be7128997a0aa89b75885cc4ad4c82e/pgconn/config.go#L218
	v.SetDefault("pg.ssl.mode", "disable")
}

func parseDBConfig(v *viper.Viper, cfg *Prest) error {
	cfg.Engine = normalizeEngine(v.GetString("engine"))
	if cfg.Engine == "" {
		cfg.Engine = EnginePostgres
	}
	if cfg.Engine == EngineMySQL {
		return parseMySQLDBConfig(v, cfg)
	}
	cfg.PGURL = v.GetString("pg.url")
	cfg.PGHost = v.GetString("pg.host")
	cfg.PGPort = v.GetInt("pg.port")
	cfg.PGUser = v.GetString("pg.user")
	cfg.PGPass = v.GetString("pg.pass")
	cfg.PGDatabase = v.GetString("pg.database")
	cfg.PGSSLMode = v.GetString("pg.ssl.mode")
	cfg.PGSSLKey = v.GetString("pg.ssl.key")
	cfg.PGSSLCert = v.GetString("pg.ssl.cert")
	cfg.PGSSLRootCert = v.GetString("pg.ssl.rootcert")

	if os.Getenv("DATABASE_URL") != "" {
		// cloud factor support: https://devcenter.heroku.com/changelog-items/438
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	parseDatabaseURL(cfg)

	cfg.PGMaxIdleConn = v.GetInt("pg.maxidleconn")
	cfg.PGMaxOpenConn = v.GetInt("pg.maxopenconn")
	cfg.PGConnTimeout = v.GetInt("pg.conntimeout")
	cfg.PGCache = v.GetBool("pg.cache")
	cfg.SingleDB = v.GetBool("pg.single")
	return nil
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

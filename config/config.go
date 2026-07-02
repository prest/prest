package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/structy/log"

	"log/slog"

	"github.com/lestrrat-go/jwx/v2/jwk"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

const (
	jsonAggDefault = "jsonb_agg"
	jsonAgg        = "json_agg"
)

// TablesConf informations

type TablesConf struct {
	Name        string   `mapstructure:"name"`
	Permissions []string `mapstructure:"permissions"`
	Fields      []string `mapstructure:"fields"`
}

type UsersConf struct {
	Name   string `mapstructure:"name"`
	Tables []TablesConf
}

// AccessConf informations
type AccessConf struct {
	Restrict    bool
	IgnoreTable []string
	Tables      []TablesConf
	Users       []UsersConf
}

// ExposeConf (expose data) information
type ExposeConf struct {
	Enabled         bool
	DatabaseListing bool
	SchemaListing   bool
	TableListing    bool
}

type PluginMiddleware struct {
	File string
	Func string
}

// Prest basic config
type Prest struct {
	AuthEnabled          bool
	AuthSchema           string
	AuthTable            string
	AuthUsername         string
	AuthPassword         string
	AuthEncrypt          string
	AuthMetadata         []string
	AuthType             string
	HTTPHost             string // HTTPHost Declare which http address the PREST used
	HTTPPort             int    // HTTPPort Declare which http port the PREST used
	HTTPTimeout          int
	PGHost               string
	PGPort               int
	PGUser               string
	PGPass               string
	PGDatabase           string
	PGURL                string
	PGSSLMode            string
	PGSSLCert            string
	PGSSLKey             string
	PGSSLRootCert        string
	ContextPath          string
	PGMaxIdleConn        int
	PGMaxOpenConn        int
	PGConnTimeout        int
	PGCache              bool
	JWTKey               string
	JWTAlgo              string
	JWTWellKnownURL      string
	JWTJWKS              string
	JWTWhiteList         []string
	JSONAggType          string
	MigrationsPath       string
	QueriesPath          string
	AccessConf           AccessConf
	ExposeConf           ExposeConf
	CORSAllowOrigin      []string
	CORSAllowHeaders     []string
	CORSAllowMethods     []string
	CORSAllowCredentials bool
	Debug                bool
	Adapter              adapters.Adapter
	EnableDefaultJWT     bool
	SingleDB             bool
	HTTPSMode            bool
	HTTPSCert            string
	HTTPSKey             string
	Cache                cache.Config
	PluginPath           string
	PluginMiddlewareList []PluginMiddleware
	Logger               *slog.Logger
}

const defaultCfgFile = "./prest.toml"

// Load configuration
func Load() (*Prest, error) {
	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)
	if _, err := os.Stat(cfg.QueriesPath); os.IsNotExist(err) {
		if err = os.MkdirAll(cfg.QueriesPath, 0700); err != nil {
			return nil, fmt.Errorf("create queries directory %q: %w", cfg.QueriesPath, err)
		}
	}

	// Cache storage is optional when cache is disabled.
	if !cfg.Cache.Enabled {
		return setupLogger(cfg)
	}

	if _, err := os.Stat(cfg.Cache.StoragePath); os.IsNotExist(err) {
		if err = os.MkdirAll(cfg.Cache.StoragePath, 0700); err != nil {
			return nil, fmt.Errorf("create cache directory %q: %w", cfg.Cache.StoragePath, err)
		}
	}

	return setupLogger(cfg)
}

func setupLogger(cfg *Prest) (*Prest, error) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	if logLevel := os.Getenv("PREST_LOG_LEVEL"); logLevel != "" {
		var l slog.Level
		if err := l.UnmarshalText([]byte(logLevel)); err == nil {
			opts.Level = l
		}
	}
	PrestdHandler := slog.NewJSONHandler(os.Stdout, opts)
	cfg.Logger = slog.New(PrestdHandler)
	slog.SetDefault(cfg.Logger)
	return cfg, nil
}

func viperCfg() (*viper.Viper, string) {
	v := viper.New()
	configPath := getPrestConfFile(os.Getenv("PREST_CONF"))

	dir, file := filepath.Split(configPath)
	file = strings.TrimSuffix(file, filepath.Ext(file))
	replacer := strings.NewReplacer(".", "_")
	v.SetEnvPrefix("PREST")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(replacer)
	v.AddConfigPath(dir)
	v.SetConfigName(file)
	v.SetConfigType("toml")

	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.username", "username")
	v.SetDefault("auth.password", "password")
	v.SetDefault("auth.schema", "public")
	v.SetDefault("auth.table", "prest_users")
	v.SetDefault("auth.encrypt", "bcrypt")
	v.SetDefault("auth.type", "body")

	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 3000)
	v.SetDefault("http.timeout", 60)

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

	v.SetDefault("jwt.default", true)
	v.SetDefault("jwt.algo", "HS256")
	v.SetDefault("jwt.wellknownurl", "")
	v.SetDefault("jwt.jwks", "")
	v.SetDefault("jwt.whitelist", []string{`^\/auth$`})

	v.SetDefault("json.agg.type", "jsonb_agg")

	v.SetDefault("cors.allowheaders", []string{"Content-Type"})
	v.SetDefault("cors.allowmethods", []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.alloworigin", []string{"*"})
	v.SetDefault("cors.allowcredentials", true)

	v.SetDefault("https.mode", false)
	v.SetDefault("https.cert", "/etc/certs/cert.crt")
	v.SetDefault("https.key", "/etc/certs/cert.key")

	v.SetDefault("cache.enabled", false)
	v.SetDefault("cache.time", 10)
	v.SetDefault("cache.storagepath", "./")
	v.SetDefault("cache.sufixfile", ".cache.prestd.db")

	v.SetDefault("version", 1)
	v.SetDefault("debug", false)
	v.SetDefault("context", "/")
	v.SetDefault("pluginpath", "./lib")
	v.SetDefault("pluginmiddlewarelist", []PluginMiddleware{})
	v.SetDefault("expose.enabled", false)
	v.SetDefault("expose.tables", true)
	v.SetDefault("expose.schemas", true)
	v.SetDefault("expose.databases", true)

	hDir, err := homedir.Dir()
	if err != nil {
		slog.Error("could not find homedir", "err", err)
	} else {
		v.SetDefault("queries.location", filepath.Join(hDir, "queries"))
	}
	return v, configPath
}

func getPrestConfFile(prestConf string) string {
	if prestConf != "" {
		return prestConf
	}
	return defaultCfgFile
}

// Parse pREST config
// todo: split config onto methods to simplify this
func Parse(v *viper.Viper, cfg *Prest, configPath string) {
	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("file not found, falling back to default settings", "file", configPath)
			cfg.PGSSLMode = "disable"
		}
		slog.Warn("read env config error", "err", err)
	}

	parseAuthConfig(v, cfg)
	parseHTTPConfig(v, cfg)
	portFromEnv(cfg)
	parseDBConfig(v, cfg)

	cfg.JWTKey = v.GetString("jwt.key")
	cfg.JWTAlgo = v.GetString("jwt.algo")
	cfg.JWTWellKnownURL = v.GetString("jwt.wellknownurl")
	cfg.JWTJWKS = v.GetString("jwt.jwks")
	cfg.JWTWhiteList = v.GetStringSlice("jwt.whitelist")
	fetchJWKS(cfg)

	cfg.JSONAggType = getJSONAgg(v)

	cfg.MigrationsPath = v.GetString("migrations")

	cfg.AccessConf.Restrict = v.GetBool("access.restrict")
	cfg.AccessConf.IgnoreTable = v.GetStringSlice("access.ignore_table")
	cfg.QueriesPath = v.GetString("queries.location")

	cfg.CORSAllowOrigin = v.GetStringSlice("cors.alloworigin")
	cfg.CORSAllowHeaders = v.GetStringSlice("cors.allowheaders")
	cfg.CORSAllowMethods = v.GetStringSlice("cors.allowmethods")
	cfg.CORSAllowCredentials = v.GetBool("cors.allowcredentials")

	cfg.Debug = v.GetBool("debug")
	cfg.EnableDefaultJWT = v.GetBool("jwt.default")
	cfg.ContextPath = v.GetString("context")

	cfg.PluginPath = v.GetString("pluginpath")

	loadCacheConfig(v, cfg)

	cfg.ExposeConf.Enabled = v.GetBool("expose.enabled")
	cfg.ExposeConf.TableListing = v.GetBool("expose.tables")
	cfg.ExposeConf.SchemaListing = v.GetBool("expose.schemas")
	cfg.ExposeConf.DatabaseListing = v.GetBool("expose.databases")

	// table access config
	var tablesconf []TablesConf
	err = v.UnmarshalKey("access.tables", &tablesconf)
	if err != nil {
		slog.Error("could not unmarshal access tables", "err", err)
	}
	cfg.AccessConf.Tables = tablesconf

	var usersconf []UsersConf
	err = v.UnmarshalKey("access.users", &usersconf)
	if err != nil {
		slog.Error("could not unmarshal access users", "err", err)
	}
	cfg.AccessConf.Users = usersconf

	// plugin middleware list config
	var pluginMiddlewareConfig []PluginMiddleware
	err = v.UnmarshalKey("pluginmiddlewarelist", &pluginMiddlewareConfig)
	if err != nil {
		slog.Error("could not unmarshal access plugin middleware list", "err", err)
	}
	cfg.PluginMiddlewareList = pluginMiddlewareConfig
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

// ErrJWTDefaultEnabledNoKey is returned when the default JWT middleware is
// enabled but no verification material (HMAC key, JWKS or .well-known URL) was
// provided. This guards against accidentally serving requests with an empty
// HMAC key, which would let any client forge bearer tokens. See GHSA-fj7v-859r-2fm4.
var ErrJWTDefaultEnabledNoKey = errors.New(
	"jwt.default is enabled but no verification material was provided " +
		"(set jwt.key, jwt.jwks or jwt.wellknownurl, or disable jwt.default)")

// ErrAuthEnabledNoJWTKey is returned when basic auth is enabled but jwt.key
// is empty. AuthMiddleware uses the same []byte(JWTKey) to verify HS256
// tokens, so an empty key opens the same auth-bypass as the default JWT
// middleware. See GHSA-fj7v-859r-2fm4.
var ErrAuthEnabledNoJWTKey = errors.New(
	"auth.enabled is true but jwt.key is empty (required to verify HS256 tokens)")

// ValidateJWTConfig fails fast when either of the JWT-validating middlewares
// would be installed without any verification material:
//
//   - The default JWT middleware (jwt.default = true) requires jwt.key, a
//     JWKS, or a .well-known URL.
//   - AuthMiddleware (auth.enabled = true) verifies HS256 tokens with
//     jwt.key, so an empty key is unsafe.
//
// The default JWT path also bypasses when Debug is true, so we mirror that
// rule here to avoid blocking debug-mode startups.
//
// Call this from binary entrypoints before serving requests; tests that
// exercise Load() without setting JWT material rely on the middleware-level
// guards (middlewares.JwtMiddleware, middlewares.AuthMiddleware) to fail
// closed at request time.
func ValidateJWTConfig(cfg *Prest) error {
	if cfg.AuthEnabled && cfg.JWTKey == "" {
		return ErrAuthEnabledNoJWTKey
	}
	if !cfg.EnableDefaultJWT {
		return nil
	}
	if cfg.Debug {
		return nil
	}
	if cfg.JWTKey != "" || cfg.JWTJWKS != "" || cfg.JWTWellKnownURL != "" {
		return nil
	}
	return ErrJWTDefaultEnabledNoKey
}

// fetchJWKS tries to get the JWKS from the URL in the config
func fetchJWKS(cfg *Prest) {
	if cfg.JWTWellKnownURL == "" {
		slog.Debug("no JWT WellKnown url found, skipping")
		return
	}
	if cfg.JWTJWKS != "" {
		slog.Debug("JWKS already set, skipping")
		return
	}

	// Call provider to obtain .well-known config
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	r, err := client.Get(cfg.JWTWellKnownURL)
	if err != nil {
		slog.Error("Cannot get .well-known configuration", "url", cfg.JWTWellKnownURL, "err", err)
		return
	}
	defer r.Body.Close()

	var wellKnown map[string]interface{}
	err = json.NewDecoder(r.Body).Decode(&wellKnown)
	if err != nil {
		slog.Error("Failed to decode JSON", "err", err)
		return
	}

	//Retrieve the JWKS from the endpoint
	uri, ok := wellKnown["jwks_uri"].(string)
	if !ok {
		slog.Error("Unable to convert .WellKnown configuration of jwks_uri to a string")
		return
	}

	JWKSet, err := jwk.Fetch(context.Background(), uri)
	if err != nil {
		err := fmt.Errorf("failed to parse JWK: %s", err)
		log.Errorf("Failed to fetch JWK: %v\n", err)
		return
	}

	//Convert set to json string
	jwkSetJSON, err := json.Marshal(JWKSet)
	if err != nil {
		slog.Error("Failed to marshal JWKSet to JSON", "err", err)
		return
	}

	cfg.JWTJWKS = string(jwkSetJSON)
}

func portFromEnv(cfg *Prest) {
	if os.Getenv("PORT") == "" {
		slog.Debug("could not find PORT in env")
		return
	}
	// cloud factor support: https://help.heroku.com/PPBPA231/how-do-i-use-the-port-environment-variable-in-container-based-apps
	HTTPPort, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		slog.Debug("could not find PORT in env")
		return
	}
	cfg.HTTPPort = HTTPPort
}

// getJSONAgg identifies which json aggregation function will be used,
// support `jsonb` and `json`; `jsonb` is the default value
//
// https://www.postgresql.org/docs/9.5/functions-aggregate.html
func getJSONAgg(v *viper.Viper) (config string) {
	config = v.GetString("json.agg.type")
	if config == jsonAgg {
		return jsonAgg
	}
	if config != jsonAggDefault {
		slog.Warn("JSON Agg type can only be 'json_agg' or 'jsonb_agg', using the later as default")
	}
	return jsonAggDefault
}

func parseDBConfig(v *viper.Viper, cfg *Prest) {
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
}

func loadCacheConfig(v *viper.Viper, cfg *Prest) {
	cfg.Cache.Enabled = v.GetBool("cache.enabled")
	cfg.Cache.Time = v.GetInt("cache.time")
	cfg.Cache.StoragePath = v.GetString("cache.storagepath")
	cfg.Cache.SufixFile = v.GetString("cache.sufixfile")

	// cache endpoints config
	var cacheendpoints = []cache.Endpoint{}
	err := v.UnmarshalKey("cache.endpoints", &cacheendpoints)
	if err != nil {
		slog.Error("could not unmarshal cache endpoints", "err", err)
	}
	cfg.Cache.Endpoints = cacheendpoints
}

func parseAuthConfig(v *viper.Viper, cfg *Prest) {
	cfg.AuthEnabled = v.GetBool("auth.enabled")
	cfg.AuthSchema = v.GetString("auth.schema")
	cfg.AuthTable = v.GetString("auth.table")
	cfg.AuthUsername = v.GetString("auth.username")
	cfg.AuthPassword = v.GetString("auth.password")
	cfg.AuthEncrypt = v.GetString("auth.encrypt")
	cfg.AuthMetadata = v.GetStringSlice("auth.metadata")
	cfg.AuthType = v.GetString("auth.type")
}

func parseHTTPConfig(v *viper.Viper, cfg *Prest) {
	cfg.HTTPHost = v.GetString("http.host")
	cfg.HTTPPort = v.GetInt("http.port")
	cfg.HTTPTimeout = v.GetInt("http.timeout")

	cfg.HTTPSMode = v.GetBool("https.mode")
	cfg.HTTPSCert = v.GetString("https.cert")
	cfg.HTTPSKey = v.GetString("https.key")
}

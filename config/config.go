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
	Database    string   `mapstructure:"database"`
	Schema      string   `mapstructure:"schema"`
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
	AuthMigrateOnStartup bool
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
	QueriesConf          QueriesConf
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
	Databases            []DatabaseConf
	HTTPSMode            bool
	HTTPSCert            string
	HTTPSKey             string
	Cache                cache.Config
	PluginPath           string
	PluginMiddlewareList []PluginMiddleware
	Logger               *slog.Logger
}

const (
	defaultCfgFile          = "./prest.toml"
	defaultCacheStoragePath = "./"
)

// Load reads pREST configuration from the TOML file named by PREST_CONF, or
// ./prest.toml when that variable is unset. Environment variables with the
// PREST_ prefix override file values (keys use underscores instead of dots).
//
// It populates a Prest via Parse, ensures the queries directory when possible,
// and when cache is enabled ensures the cache storage directory exists.
// On success it configures cfg.Logger and the process-wide default logger via
// setupLogger (level debug, overridable with PREST_LOG_LEVEL).
//
// Parse logs warnings and falls back to viper defaults when the config file
// is missing, unreadable, malformed, or contains invalid structured keys.
// Load never returns an error for queries or cache storage path issues: it
// retries default paths and disables the feature when both configured and
// fallback paths are unavailable. Unsafe JWT/auth settings (enabled without
// verification material) are auto-disabled with warnings via ensureJWTConfig.
// Invalid database registry entries (duplicate aliases, missing URLs, invalid
// aliases) are logged and skipped; Load never fails for registry content.
//
// Returns the populated *Prest and nil on success.
func Load() (*Prest, error) {
	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)

	parseDatabaseRegistry(v, cfg)

	ensureJWTConfig(cfg)
	ensureQueriesPath(cfg)
	ensureQueriesConfig(cfg)

	if !cfg.Cache.Enabled {
		return setupLogger(cfg)
	}

	ensureCacheStorage(cfg)

	return setupLogger(cfg)
}

// ensureDir ensures path exists as a writable directory.
// It creates missing directories, rejects non-directory paths, and verifies
// writability with a temporary test file.
func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("create directory %q: %w", path, err)
			}
		} else {
			return err
		}
	} else if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", path)
	}

	testFile := filepath.Join(path, ".prest-write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return fmt.Errorf("directory %q is not writable: %w", path, err)
	}
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("directory %q is not writable: %w", path, err)
	}
	return nil
}

// ensureCacheStorage ensures the cache storage directory exists and is writable.
// On failure it tries the default path, then disables cache.
func ensureCacheStorage(cfg *Prest) {
	configuredPath := cfg.Cache.StoragePath
	err := ensureDir(configuredPath)
	if err == nil {
		return
	}

	slog.Warn("cache storage path unavailable, trying fallback", "path", configuredPath, "err", err)

	if configuredPath == defaultCacheStoragePath {
		slog.Warn("cache disabled: default storage path unavailable", "path", configuredPath, "err", err)
		cfg.Cache.Enabled = false
		return
	}

	if err = ensureDir(defaultCacheStoragePath); err == nil {
		cfg.Cache.StoragePath = defaultCacheStoragePath
		return
	}

	slog.Warn("cache disabled: fallback storage path unavailable", "path", defaultCacheStoragePath, "err", err)
	cfg.Cache.Enabled = false
}

func ensureJWTConfig(cfg *Prest) {
	if cfg.AuthEnabled && cfg.JWTKey == "" {
		slog.Error("auth disabled: jwt.key is empty", "err", ErrAuthEnabledNoJWTKey)
		cfg.AuthEnabled = false
	}
	if !cfg.EnableDefaultJWT || cfg.Debug {
		return
	}
	if cfg.JWTKey != "" || cfg.JWTJWKS != "" || cfg.JWTWellKnownURL != "" {
		return
	}
	slog.Error(
		"default JWT middleware disabled: no verification material",
		"err", ErrJWTDefaultEnabledNoKey)
	cfg.EnableDefaultJWT = false
}

func defaultQueriesPath() string {
	hDir, err := homedir.Dir()
	if err != nil {
		slog.Error("could not find homedir", "err", err)
		return filepath.Join(".", "queries")
	}
	return filepath.Join(hDir, "queries")
}

func ensureQueriesPath(cfg *Prest) {
	if cfg.QueriesConf.Storage == QueriesStorageDatabase {
		// Database mode uses prest_queries at runtime; location is import-only.
		if !cfg.QueriesConf.ImportOnStartup {
			return
		}
		if cfg.QueriesPath == "" {
			return
		}
		if err := ensureDir(cfg.QueriesPath); err != nil {
			slog.Warn("queries import path unavailable", "path", cfg.QueriesPath, "err", err)
		}
		return
	}

	configuredPath := cfg.QueriesPath
	err := ensureDir(configuredPath)
	if err == nil {
		return
	}

	slog.Warn("queries path unavailable, trying fallback", "path", configuredPath, "err", err)

	fallback := defaultQueriesPath()
	if configuredPath == fallback {
		slog.Warn("queries disabled: default queries path unavailable", "path", configuredPath, "err", err)
		cfg.QueriesPath = ""
		return
	}

	if err = ensureDir(fallback); err == nil {
		cfg.QueriesPath = fallback
		return
	}

	slog.Warn("queries disabled: fallback queries path unavailable", "path", fallback, "err", err)
	cfg.QueriesPath = ""
}

func unmarshalKeyOrZero[T any](v *viper.Viper, key string) T {
	var out T
	if err := v.UnmarshalKey(key, &out); err != nil {
		slog.Warn("config key invalid, using default", "key", key, "err", err)
		var zero T
		return zero
	}
	return out
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

	v.SetDefault("jwt.default", false)
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
	v.SetDefault("cache.storagepath", defaultCacheStoragePath)
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

	v.SetDefault("queries.location", defaultQueriesPath())
	v.SetDefault("queries.storage", QueriesStorageFilesystem)
	v.SetDefault("queries.schema", "public")
	v.SetDefault("queries.table", "prest_queries")
	v.SetDefault("queries.restrict", false)
	v.SetDefault("queries.register_enabled", false)
	v.SetDefault("queries.import_policy", QueriesImportPolicyUpdate)
	return v, configPath
}

func getPrestConfFile(prestConf string) string {
	if prestConf != "" {
		return prestConf
	}
	return defaultCfgFile
}

// Parse pREST config. Invalid or missing config files log warnings and fall
// back to viper defaults and environment overrides; structured keys that fail
// to unmarshal use zero values. Parse does not fail startup for config content.
func Parse(v *viper.Viper, cfg *Prest, configPath string) {
	if err := v.ReadInConfig(); err != nil {
		slog.Warn("config file unavailable, falling back to default settings", "file", configPath, "err", err)
		cfg.PGSSLMode = "disable"
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
	parseQueriesConfig(v, cfg)

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

	cfg.AccessConf.Tables = unmarshalKeyOrZero[[]TablesConf](v, "access.tables")
	cfg.AccessConf.Users = unmarshalKeyOrZero[[]UsersConf](v, "access.users")
	cfg.PluginMiddlewareList = unmarshalKeyOrZero[[]PluginMiddleware](v, "pluginmiddlewarelist")
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

	cfg.Cache.Endpoints = unmarshalKeyOrZero[[]cache.Endpoint](v, "cache.endpoints")
}

func parseAuthConfig(v *viper.Viper, cfg *Prest) {
	cfg.AuthEnabled = v.GetBool("auth.enabled")
	if v.IsSet("auth.migrate_on_startup") {
		cfg.AuthMigrateOnStartup = v.GetBool("auth.migrate_on_startup")
	} else {
		cfg.AuthMigrateOnStartup = cfg.AuthEnabled
	}
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

package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/cache"

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

// The listing predicates below are the single definition of what [expose]
// permits. Every surface that returns catalog metadata — the REST routes via
// ExposureMiddleware and the /_mcp tools — must consult them, so a new
// discovery endpoint cannot silently escape the control. When the section is
// disabled the flags carry no meaning and everything is listable.

// DatabaseListingAllowed reports whether database names may be listed.
func (e ExposeConf) DatabaseListingAllowed() bool {
	return !e.Enabled || e.DatabaseListing
}

// SchemaListingAllowed reports whether schema names may be listed.
func (e ExposeConf) SchemaListingAllowed() bool {
	return !e.Enabled || e.SchemaListing
}

// TableListingAllowed reports whether table names may be listed.
func (e ExposeConf) TableListingAllowed() bool {
	return !e.Enabled || e.TableListing
}

// StudioConf controls the embedded pREST Studio UI.
type StudioConf struct {
	Enabled bool
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
	// authSchemaSet and queriesSchemaSet mean the operator set those keys.
	// The viper default "public" is not explicit, so an explicit public stays.
	authSchemaSet        bool
	queriesSchemaSet     bool
	AccessConf           AccessConf
	ExposeConf           ExposeConf
	StudioConf           StudioConf
	CORSAllowOrigin      []string
	CORSAllowHeaders     []string
	CORSAllowMethods     []string
	CORSAllowCredentials bool
	Debug                bool
	Engine               string // postgres (default) or mysql
	MySQLPrepare         bool
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
	Otel                 OtelConf
	Logger               *slog.Logger
}

const defaultCfgFile = "./prest.toml"

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
	if err := Parse(v, cfg, configPath); err != nil {
		return nil, err
	}

	parseDatabaseRegistry(v, cfg)
	if err := cfg.ValidateEngines(); err != nil {
		return nil, err
	}

	ensureJWTConfig(cfg)
	ensureQueriesPath(cfg)
	ensureQueriesConfig(cfg)

	if !cfg.Cache.Enabled {
		return setupLogger(cfg)
	}

	ensureCacheStorage(cfg)

	return setupLogger(cfg)
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

	setAuthDefaults(v)
	setServerDefaults(v)
	setEngineDefaults(v)
	setMySQLDefaults(v)
	setPostgresDefaults(v)
	setJWTDefaults(v)
	setCacheDefaults(v)
	setQueriesDefaults(v)
	setOtelDefaults(v)
	return v, configPath
}

func setServerDefaults(v *viper.Viper) {
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 3000)
	v.SetDefault("http.timeout", 60)

	v.SetDefault("json.agg.type", "jsonb_agg")

	v.SetDefault("cors.allowheaders", []string{"Content-Type"})
	v.SetDefault("cors.allowmethods", []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.alloworigin", []string{"*"})
	v.SetDefault("cors.allowcredentials", true)

	v.SetDefault("https.mode", false)
	v.SetDefault("https.cert", "/etc/certs/cert.crt")
	v.SetDefault("https.key", "/etc/certs/cert.key")

	v.SetDefault("version", 1)
	v.SetDefault("debug", false)
	v.SetDefault("context", "/")
	v.SetDefault("pluginpath", "./lib")
	v.SetDefault("pluginmiddlewarelist", []PluginMiddleware{})
	v.SetDefault("expose.enabled", false)
	v.SetDefault("expose.tables", true)
	v.SetDefault("expose.schemas", true)
	v.SetDefault("expose.databases", true)

	v.SetDefault("studio.enabled", true)
}

func getPrestConfFile(prestConf string) string {
	if prestConf != "" {
		return prestConf
	}
	return defaultCfgFile
}

// Parse pREST config. Invalid or missing config files log warnings and fall
// back to viper defaults and environment overrides; structured keys that fail
// to unmarshal use zero values. A mysql URL that cannot be parsed, including
// a non-numeric or overflowing port, returns an error. Other config content
// does not fail startup.
func Parse(v *viper.Viper, cfg *Prest, configPath string) error {
	if err := v.ReadInConfig(); err != nil {
		slog.Warn("config file unavailable, falling back to default settings", "file", configPath, "err", err)
		cfg.PGSSLMode = "disable"
	}

	parseAuthConfig(v, cfg)
	parseHTTPConfig(v, cfg)
	portFromEnv(cfg)
	if err := parseDBConfig(v, cfg); err != nil {
		return err
	}

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
	cfg.authSchemaSet = mysqlConfigured(v, "auth.schema")
	cfg.queriesSchemaSet = mysqlConfigured(v, "queries.schema")
	applyMySQLSchemaDefaults(v, cfg)

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

	cfg.StudioConf.Enabled = v.GetBool("studio.enabled")

	parseOtelConfig(v, cfg)

	cfg.AccessConf.Tables = unmarshalKeyOrZero[[]TablesConf](v, "access.tables")
	cfg.AccessConf.Users = unmarshalKeyOrZero[[]UsersConf](v, "access.users")
	cfg.PluginMiddlewareList = unmarshalKeyOrZero[[]PluginMiddleware](v, "pluginmiddlewarelist")
	return nil
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

func parseHTTPConfig(v *viper.Viper, cfg *Prest) {
	cfg.HTTPHost = v.GetString("http.host")
	cfg.HTTPPort = v.GetInt("http.port")
	cfg.HTTPTimeout = v.GetInt("http.timeout")

	cfg.HTTPSMode = v.GetBool("https.mode")
	cfg.HTTPSCert = v.GetString("https.cert")
	cfg.HTTPSKey = v.GetString("https.key")
}

package config

import (
	"context"
	"encoding/json"
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

const defaultCacheDir = "./"

var (
	// PrestConf config variable
	PrestConf      *Prest
	configFile     string
	defaultCfgFile = "./prest.toml"
)

// Load configuration
func Load() {
	viperCfg()
	PrestConf = &Prest{}
	Parse(PrestConf)
	if _, err := os.Stat(PrestConf.QueriesPath); os.IsNotExist(err) {
		if err = os.MkdirAll(PrestConf.QueriesPath, 0700); err != nil {
			slog.Error("Queries directory was not created", "path", PrestConf.QueriesPath, "err", err)
		}
	}

	// ignore cache if disabled
	if !PrestConf.Cache.Enabled {
		return
	}

	if _, err := os.Stat(PrestConf.Cache.StoragePath); os.IsNotExist(err) {
		if err = os.MkdirAll(PrestConf.Cache.StoragePath, 0700); err != nil {
			slog.Error("Cache directory was not created, falling back to default './'", "path", PrestConf.Cache.StoragePath, "err", err)
			PrestConf.Cache.StoragePath = defaultCacheDir
		}
	}

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
	PrestConf.Logger = slog.New(PrestdHandler)
	slog.SetDefault(PrestConf.Logger)
}

func viperCfg() {
	configFile = getPrestConfFile(os.Getenv("PREST_CONF"))

	dir, file := filepath.Split(configFile)
	file = strings.TrimSuffix(file, filepath.Ext(file))
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvPrefix("PREST")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(replacer)
	viper.AddConfigPath(dir)
	viper.SetConfigName(file)
	viper.SetConfigType("toml")

	viper.SetDefault("auth.enabled", false)
	viper.SetDefault("auth.username", "username")
	viper.SetDefault("auth.password", "password")
	viper.SetDefault("auth.schema", "public")
	viper.SetDefault("auth.table", "prest_users")
	viper.SetDefault("auth.encrypt", "MD5")
	viper.SetDefault("auth.type", "body")

	viper.SetDefault("http.host", "0.0.0.0")
	viper.SetDefault("http.port", 3000)
	viper.SetDefault("http.timeout", 60)

	viper.SetDefault("pg.host", "127.0.0.1")
	viper.SetDefault("pg.port", 5432)
	viper.SetDefault("pg.database", "prest")
	viper.SetDefault("pg.user", "postgres")
	viper.SetDefault("pg.pass", "postgres")
	viper.SetDefault("pg.maxidleconn", 0) // avoids db memory leak on req timeout
	viper.SetDefault("pg.maxopenconn", 10)
	viper.SetDefault("pg.conntimeout", 10)
	viper.SetDefault("pg.single", true)
	viper.SetDefault("pg.cache", true)
	// todo: replace this with prefer, will need to replace lib/pq
	// https://github.com/jackc/pgx/blob/47d631e34be7128997a0aa89b75885cc4ad4c82e/pgconn/config.go#L218
	viper.SetDefault("pg.ssl.mode", "disable")

	viper.SetDefault("jwt.default", true)
	viper.SetDefault("jwt.algo", "HS256")
	viper.SetDefault("jwt.wellknownurl", "")
	viper.SetDefault("jwt.jwks", "")
	viper.SetDefault("jwt.whitelist", []string{`^\/auth$`})

	viper.SetDefault("json.agg.type", "jsonb_agg")

	viper.SetDefault("cors.allowheaders", []string{"Content-Type"})
	viper.SetDefault("cors.allowmethods", []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.alloworigin", []string{"*"})
	viper.SetDefault("cors.allowcredentials", true)

	viper.SetDefault("https.mode", false)
	viper.SetDefault("https.cert", "/etc/certs/cert.crt")
	viper.SetDefault("https.key", "/etc/certs/cert.key")

	viper.SetDefault("cache.enabled", false)
	viper.SetDefault("cache.time", 10)
	viper.SetDefault("cache.storagepath", "./")
	viper.SetDefault("cache.sufixfile", ".cache.prestd.db")

	viper.SetDefault("version", 1)
	viper.SetDefault("debug", false)
	viper.SetDefault("context", "/")
	viper.SetDefault("pluginpath", "./lib")
	viper.SetDefault("pluginmiddlewarelist", []PluginMiddleware{})
	viper.SetDefault("expose.enabled", false)
	viper.SetDefault("expose.tables", true)
	viper.SetDefault("expose.schemas", true)
	viper.SetDefault("expose.databases", true)

	hDir, err := homedir.Dir()
	if err != nil {
		slog.Error("could not find homedir", "err", err)
		os.Exit(1)
	}
	viper.SetDefault("queries.location", filepath.Join(hDir, "queries"))
}

func getPrestConfFile(prestConf string) string {
	if prestConf != "" {
		return prestConf
	}
	return defaultCfgFile
}

// Parse pREST config
// todo: split config onto methods to simplify this
func Parse(cfg *Prest) {
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("file not found, falling back to default settings", "file", configFile)
			cfg.PGSSLMode = "disable"
		}
		slog.Warn("read env config error", "err", err)
	}

	parseAuthConfig(cfg)
	parseHTTPConfig(cfg)
	portFromEnv(cfg)
	parseDBConfig(cfg)

	cfg.JWTKey = viper.GetString("jwt.key")
	cfg.JWTAlgo = viper.GetString("jwt.algo")
	cfg.JWTWellKnownURL = viper.GetString("jwt.wellknownurl")
	cfg.JWTJWKS = viper.GetString("jwt.jwks")
	cfg.JWTWhiteList = viper.GetStringSlice("jwt.whitelist")
	fetchJWKS(cfg)

	cfg.JSONAggType = getJSONAgg()

	cfg.MigrationsPath = viper.GetString("migrations")

	cfg.AccessConf.Restrict = viper.GetBool("access.restrict")
	cfg.AccessConf.IgnoreTable = viper.GetStringSlice("access.ignore_table")
	cfg.QueriesPath = viper.GetString("queries.location")

	cfg.CORSAllowOrigin = viper.GetStringSlice("cors.alloworigin")
	cfg.CORSAllowHeaders = viper.GetStringSlice("cors.allowheaders")
	cfg.CORSAllowMethods = viper.GetStringSlice("cors.allowmethods")
	cfg.CORSAllowCredentials = viper.GetBool("cors.allowcredentials")

	cfg.Debug = viper.GetBool("debug")
	cfg.EnableDefaultJWT = viper.GetBool("jwt.default")
	cfg.ContextPath = viper.GetString("context")

	cfg.PluginPath = viper.GetString("pluginpath")

	loadCacheConfig(cfg)

	cfg.ExposeConf.Enabled = viper.GetBool("expose.enabled")
	cfg.ExposeConf.TableListing = viper.GetBool("expose.tables")
	cfg.ExposeConf.SchemaListing = viper.GetBool("expose.schemas")
	cfg.ExposeConf.DatabaseListing = viper.GetBool("expose.databases")

	// table access config
	var tablesconf []TablesConf
	err = viper.UnmarshalKey("access.tables", &tablesconf)
	if err != nil {
		slog.Error("could not unmarshal access tables", "err", err)
	}
	cfg.AccessConf.Tables = tablesconf

	var usersconf []UsersConf
	err = viper.UnmarshalKey("access.users", &usersconf)
	if err != nil {
		slog.Error("could not unmarshal access users", "err", err)
	}
	cfg.AccessConf.Users = usersconf

	// plugin middleware list config
	var pluginMiddlewareConfig []PluginMiddleware
	err = viper.UnmarshalKey("pluginmiddlewarelist", &pluginMiddlewareConfig)
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
		slog.Error("cannot parse db url", "err", err)
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
func getJSONAgg() (config string) {
	config = viper.GetString("json.agg.type")
	if config == jsonAgg {
		return jsonAgg
	}
	if config != jsonAggDefault {
		slog.Warn("JSON Agg type can only be 'json_agg' or 'jsonb_agg', using the later as default")
	}
	return jsonAggDefault
}

func parseDBConfig(cfg *Prest) {
	cfg.PGURL = viper.GetString("pg.url")
	cfg.PGHost = viper.GetString("pg.host")
	cfg.PGPort = viper.GetInt("pg.port")
	cfg.PGUser = viper.GetString("pg.user")
	cfg.PGPass = viper.GetString("pg.pass")
	cfg.PGDatabase = viper.GetString("pg.database")
	cfg.PGSSLMode = viper.GetString("pg.ssl.mode")
	cfg.PGSSLKey = viper.GetString("pg.ssl.key")
	cfg.PGSSLCert = viper.GetString("pg.ssl.cert")
	cfg.PGSSLRootCert = viper.GetString("pg.ssl.rootcert")

	if os.Getenv("DATABASE_URL") != "" {
		// cloud factor support: https://devcenter.heroku.com/changelog-items/438
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	parseDatabaseURL(cfg)

	cfg.PGMaxIdleConn = viper.GetInt("pg.maxidleconn")
	cfg.PGMaxOpenConn = viper.GetInt("pg.maxopenconn")
	cfg.PGConnTimeout = viper.GetInt("pg.conntimeout")
	cfg.PGCache = viper.GetBool("pg.cache")
	cfg.SingleDB = viper.GetBool("pg.single")
}

func loadCacheConfig(cfg *Prest) {
	cfg.Cache.Enabled = viper.GetBool("cache.enabled")
	cfg.Cache.Time = viper.GetInt("cache.time")
	cfg.Cache.StoragePath = viper.GetString("cache.storagepath")
	cfg.Cache.SufixFile = viper.GetString("cache.sufixfile")

	// cache endpoints config
	var cacheendpoints = []cache.Endpoint{}
	err := viper.UnmarshalKey("cache.endpoints", &cacheendpoints)
	if err != nil {
		slog.Error("could not unmarshal cache endpoints", "err", err)
	}
	cfg.Cache.Endpoints = cacheendpoints
}

func parseAuthConfig(cfg *Prest) {
	cfg.AuthEnabled = viper.GetBool("auth.enabled")
	cfg.AuthSchema = viper.GetString("auth.schema")
	cfg.AuthTable = viper.GetString("auth.table")
	cfg.AuthUsername = viper.GetString("auth.username")
	cfg.AuthPassword = viper.GetString("auth.password")
	cfg.AuthEncrypt = viper.GetString("auth.encrypt")
	cfg.AuthMetadata = viper.GetStringSlice("auth.metadata")
	cfg.AuthType = viper.GetString("auth.type")
}

func parseHTTPConfig(cfg *Prest) {
	cfg.HTTPHost = viper.GetString("http.host")
	cfg.HTTPPort = viper.GetInt("http.port")
	cfg.HTTPTimeout = viper.GetInt("http.timeout")

	cfg.HTTPSMode = viper.GetBool("https.mode")
	cfg.HTTPSCert = viper.GetString("https.cert")
	cfg.HTTPSKey = viper.GetString("https.key")
}

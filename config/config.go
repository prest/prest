package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prest/prest/cache"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"github.com/structy/log"
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

// AccessConf informations
type AccessConf struct {
	Restrict    bool
	IgnoreTable []string
	Tables      []TablesConf
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

// Prest basic configuration
type Prest struct {
	// general app config
	Version        int
	Debug          bool
	Adapter        string
	MigrationsPath string
	QueriesPath    string
	SingleDB       bool
	Cache          cache.Config
	ExposeConf     ExposeConf
	AccessConf     AccessConf

	// auth configuration
	AuthEnabled  bool
	AuthSchema   string
	AuthTable    string
	AuthUsername string
	AuthPassword string
	AuthEncrypt  string
	AuthMetadata []string
	AuthType     string

	// HTTP server configuration
	ContextPath          string
	HTTPHost             string // host to be used
	HTTPPort             int    // port to be used
	HTTPTimeout          int
	HTTPSMode            bool
	HTTPSCert            string
	HTTPSKey             string
	CORSAllowOrigin      []string
	CORSAllowHeaders     []string
	CORSAllowMethods     []string
	CORSAllowCredentials bool

	// postgres connection configuration
	PGHost        string
	PGPort        int
	PGUser        string
	PGPass        string
	PGDatabase    string
	PGURL         string
	PGSSLMode     string
	PGSSLCert     string
	PGSSLKey      string
	PGSSLRootCert string
	PGMaxIdleConn int
	PGMaxOpenConn int
	PGConnTimeout int
	// PGConnMaxLifetime int
	PGCache     bool
	JSONAggType string

	// jwt configuration
	JWTKey           string
	EnableDefaultJWT bool
	JWTAlgo          string
	JWTWhiteList     []string

	// plugin config
	PluginPath           string
	PluginMiddlewareList []PluginMiddleware
}

func New() *Prest {
	configureViperCmd()
	cfg := &Prest{}
	Parse(cfg)
	createMigrationPath(cfg.QueriesPath)
	return cfg
}

func configureViperCmd() {
	dir, file := filepath.Split(getPrestConfFile(os.Getenv("PREST_CONF")))
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
	viper.SetDefault("pg.ssl.mode", "require")

	viper.SetDefault("jwt.default", true)
	viper.SetDefault("jwt.algo", "HS256")
	viper.SetDefault("jwt.whitelist", []string{"/auth"})

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

	viper.SetDefault("version", 2)
	viper.SetDefault("adapter", "postgres")
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
		log.Fatal(err)
	}
	viper.SetDefault("queries.location", filepath.Join(hDir, "queries"))
}

// getPrestConfFile returns the path to the config file
//
// If PREST_CONF is set, it will use that value
// if not, it will use the default value
// that is ./prest.toml
func getPrestConfFile(prestConf string) string {
	if prestConf != "" {
		return prestConf
	}
	return "./prest.toml"
}

// Parse pREST config
// todo: split config onto methods to simplify this
func Parse(cfg *Prest) {
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warningln("config file not found, falling back to default settings, err: ", err)
		}
	}
	cfg.AuthEnabled = viper.GetBool("auth.enabled")
	cfg.AuthSchema = viper.GetString("auth.schema")
	cfg.AuthTable = viper.GetString("auth.table")
	cfg.AuthUsername = viper.GetString("auth.username")
	cfg.AuthPassword = viper.GetString("auth.password")
	cfg.AuthEncrypt = viper.GetString("auth.encrypt")
	cfg.AuthMetadata = viper.GetStringSlice("auth.metadata")
	cfg.AuthType = viper.GetString("auth.type")
	cfg.HTTPHost = viper.GetString("http.host")
	cfg.HTTPPort = viper.GetInt("http.port")
	portFromEnv(cfg)
	cfg.HTTPTimeout = viper.GetInt("http.timeout")
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

	cfg.Adapter = viper.GetString("adapter")
	cfg.Version = viper.GetInt("version")

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
	cfg.JWTKey = viper.GetString("jwt.key")
	cfg.JWTAlgo = viper.GetString("jwt.algo")
	cfg.JWTWhiteList = viper.GetStringSlice("jwt.whitelist")

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
	cfg.HTTPSMode = viper.GetBool("https.mode")
	cfg.HTTPSCert = viper.GetString("https.cert")
	cfg.HTTPSKey = viper.GetString("https.key")
	cfg.PluginPath = viper.GetString("pluginpath")
	cfg.Cache.Enabled = viper.GetBool("cache.enabled")
	cfg.Cache.Time = viper.GetInt("cache.time")
	cfg.Cache.StoragePath = viper.GetString("cache.storagepath")
	cfg.Cache.SufixFile = viper.GetString("cache.sufixfile")
	cfg.ExposeConf.Enabled = viper.GetBool("expose.enabled")
	cfg.ExposeConf.TableListing = viper.GetBool("expose.tables")
	cfg.ExposeConf.SchemaListing = viper.GetBool("expose.schemas")
	cfg.ExposeConf.DatabaseListing = viper.GetBool("expose.databases")

	// cache endpoints config
	var cacheendpoints = []cache.Endpoint{}
	err = viper.UnmarshalKey("cache.endpoints", &cacheendpoints)
	if err != nil {
		log.Errorln("could not unmarshal cache endpoints")
	}
	cfg.Cache.Endpoints = cacheendpoints

	// table access config
	var tablesconf []TablesConf
	err = viper.UnmarshalKey("access.tables", &tablesconf)
	if err != nil {
		log.Errorln("could not unmarshal access tables")
	}
	cfg.AccessConf.Tables = tablesconf

	// plugin middleware list config
	var pluginMiddlewareConfig []PluginMiddleware
	err = viper.UnmarshalKey("pluginmiddlewarelist", &pluginMiddlewareConfig)
	if err != nil {
		log.Errorln("could not unmarshal access plugin middleware list")
	}
	cfg.PluginMiddlewareList = pluginMiddlewareConfig
}

// parseDatabaseURL tries to get from URL the DB configs
func parseDatabaseURL(cfg *Prest) {
	if cfg.PGURL == "" {
		log.Debugln("no db url found, skipping")
		return
	}
	// Parser PG URL, get database connection via string URL
	u, err := url.Parse(cfg.PGURL)
	if err != nil {
		log.Errorf("cannot parse db url, err: %v\n", err)
		return
	}
	cfg.PGHost = u.Hostname()
	if u.Port() != "" {
		pgPort, err := strconv.Atoi(u.Port())
		if err != nil {
			log.Errorf(
				"cannot parse db url port '%v', falling back to default values\n",
				u.Port())
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

func portFromEnv(cfg *Prest) {
	if os.Getenv("PORT") == "" {
		log.Debugln("could not find PORT in env")
		return
	}
	// cloud factor support: https://help.heroku.com/PPBPA231/how-do-i-use-the-port-environment-variable-in-container-based-apps
	HTTPPort, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		log.Debugln("could not find PORT in env")
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
		log.Warningln("JSON Agg type can only be 'json_agg' or 'jsonb_agg', using the later as default.")
	}
	return jsonAggDefault
}

func createMigrationPath(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, 0700); os.IsNotExist(err) {
			log.Errorf("Queries directory %s was not created\n", path)
		}
	}
}

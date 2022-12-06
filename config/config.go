package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/prest/prest/adapters"
	"github.com/spf13/viper"
	"github.com/structy/log"
)

// TablesConf informations
type TablesConf struct {
	Name        string   `mapstructure:"name"`
	Permissions []string `mapstructure:"permissions"`
	Fields      []string `mapstructure:"fields"`
}

// Cache structure for storing cache system configuration
type Cache struct {
	Enabled     bool            `mapstructure:"enabled"`
	Time        int             `mapstructure:"time"`
	StoragePath string          `mapstructure:"storagepath"`
	SufixFile   string          `mapstructure:"sufixfile"`
	Endpoints   []CacheEndpoint `mapstructure:"endpoints"`
}

// CacheEndpoint specific configuration for specific endpoint
type CacheEndpoint struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Time     int    `mapstructure:"time"`
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
	ContextPath          string
	SSLMode              string
	SSLCert              string
	SSLKey               string
	SSLRootCert          string
	PGMaxIdleConn        int
	PGMAxOpenConn        int
	PGConnTimeout        int
	PGCache              bool
	JWTKey               string
	JWTAlgo              string
	JWTWhiteList         []string
	MigrationsPath       string
	QueriesPath          string
	AccessConf           AccessConf
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
	Cache                Cache
	PluginPath           string
	ExposeConf           ExposeConf
}

var (
	// PrestConf config variable
	PrestConf   *Prest
	configFile  string
	defaultFile = "./prest.toml"
)

func viperCfg() {
	configFile = getDefaultPrestConf(os.Getenv("PREST_CONF"))

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
	viper.SetDefault("ssl.mode", "require")
	viper.SetDefault("pg.maxidleconn", 0) // avoids db memory leak on req timeout
	viper.SetDefault("pg.maxopenconn", 10)
	viper.SetDefault("pg.conntimeout", 10)
	viper.SetDefault("pg.single", true)
	viper.SetDefault("pg.cache", true)
	viper.SetDefault("debug", false)
	viper.SetDefault("jwt.default", true)
	viper.SetDefault("jwt.algo", "HS256")
	viper.SetDefault("jwt.whitelist", []string{"/auth"})
	viper.SetDefault("cors.allowheaders", []string{"Content-Type"})
	viper.SetDefault("cors.allowmethods", []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.alloworigin", []string{"*"})
	viper.SetDefault("cors.allowcredentials", true)
	viper.SetDefault("context", "/")
	viper.SetDefault("https.mode", false)
	viper.SetDefault("https.cert", "/etc/certs/cert.crt")
	viper.SetDefault("https.key", "/etc/certs/cert.key")
	viper.SetDefault("expose.enabled", false)
	viper.SetDefault("expose.tables", true)
	viper.SetDefault("expose.schemas", true)
	viper.SetDefault("expose.databases", true)
	viper.SetDefault("cache.enabled", false)
	viper.SetDefault("cache.time", 10)
	viper.SetDefault("cache.storagepath", "./")
	viper.SetDefault("cache.sufixfile", ".cache.prestd.db")
	viper.SetDefault("pluginpath", "./lib")

	hDir, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}
	viper.SetDefault("queries.location", filepath.Join(hDir, "queries"))
}

func getDefaultPrestConf(prestConf string) string {
	if prestConf == "" {
		return defaultFile
	}
	return prestConf
}

// Parse pREST config
func Parse(cfg *Prest) (err error) {
	err = viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("file %s not found, aborting", configFile)
		}
		return
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
	cfg.HTTPTimeout = viper.GetInt("http.timeout")
	cfg.PGURL = viper.GetString("pg.url")
	cfg.PGHost = viper.GetString("pg.host")
	cfg.PGPort = viper.GetInt("pg.port")
	cfg.PGUser = viper.GetString("pg.user")
	cfg.PGPass = viper.GetString("pg.pass")
	cfg.PGDatabase = viper.GetString("pg.database")
	cfg.SSLMode = viper.GetString("ssl.mode")
	cfg.SSLCert = viper.GetString("ssl.cert")
	cfg.SSLKey = viper.GetString("ssl.key")
	cfg.SSLRootCert = viper.GetString("ssl.rootcert")
	err = portFromEnv(cfg)
	if err != nil {
		return
	}
	if os.Getenv("DATABASE_URL") != "" {
		// cloud factor support: https://devcenter.heroku.com/changelog-items/438
		cfg.PGURL = os.Getenv("DATABASE_URL")
	}
	err = parseDatabaseURL(cfg)
	if err != nil {
		return
	}
	cfg.PGMaxIdleConn = viper.GetInt("pg.maxidleconn")
	cfg.PGMAxOpenConn = viper.GetInt("pg.maxopenconn")
	cfg.PGConnTimeout = viper.GetInt("pg.conntimeout")
	cfg.PGCache = viper.GetBool("pg.cache")
	cfg.SingleDB = viper.GetBool("pg.single")
	cfg.JWTKey = viper.GetString("jwt.key")
	cfg.JWTAlgo = viper.GetString("jwt.algo")
	cfg.JWTWhiteList = viper.GetStringSlice("jwt.whitelist")
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
	var cacheendpoints []CacheEndpoint
	err = viper.UnmarshalKey("cache.endpoints", &cacheendpoints)
	if err != nil {
		return
	}
	cfg.Cache.Endpoints = cacheendpoints
	var tablesconf []TablesConf
	err = viper.UnmarshalKey("access.tables", &tablesconf)
	if err != nil {
		return
	}
	cfg.AccessConf.Tables = tablesconf
	return
}

// Load configuration
func Load() {
	viperCfg()
	PrestConf = &Prest{}
	err := Parse(PrestConf)
	if err != nil {
		panic(err)
	}
	if _, err = os.Stat(PrestConf.QueriesPath); os.IsNotExist(err) {
		if err = os.MkdirAll(PrestConf.QueriesPath, 0700); os.IsNotExist(err) {
			log.Errorf("Queries directory %s is not created", PrestConf.QueriesPath)
		}
	}
}

func parseDatabaseURL(cfg *Prest) (err error) {
	if cfg.PGURL == "" {
		return
	}
	// Parser PG URL, get database connection via string URL
	u, errPerse := url.Parse(cfg.PGURL)
	if errPerse != nil {
		err = errPerse
		return
	}
	cfg.PGHost = u.Hostname()
	if u.Port() != "" {
		pgPort, PortErr := strconv.Atoi(u.Port())
		if PortErr != nil {
			return PortErr
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
		cfg.SSLMode = u.Query().Get("sslmode")
	}
	return
}

func portFromEnv(cfg *Prest) (err error) {
	if os.Getenv("PORT") == "" {
		return
	}
	// cloud factor support: https://help.heroku.com/PPBPA231/how-do-i-use-the-port-environment-variable-in-container-based-apps
	HTTPPort, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		return
	}
	cfg.HTTPPort = HTTPPort
	return
}

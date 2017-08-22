package config

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/nuveo/log"
	"github.com/spf13/viper"
)

// TablesConf informations
type TablesConf struct {
	Name        string   `mapstructure:"name"`
	Permissions []string `mapstructure:"permissions"`
	Fields      []string `mapstructure:"fields"`
}

// AccessConf informations
type AccessConf struct {
	Restrict bool
	Tables   []TablesConf
}

// Prest basic config
type Prest struct {
	// HTTPPort Declare which http port the PREST used
	HTTPPort         int
	PGHost           string
	PGPort           int
	PGUser           string
	PGPass           string
	PGDatabase       string
	PGMaxIdleConn    int
	PGMAxOpenConn    int
	PGConnTimeout    int
	JWTKey           string
	MigrationsPath   string
	QueriesPath      string
	AccessConf       AccessConf
	CORSAllowOrigin  []string
	CORSAllowHeaders []string
	Debug            bool
}

// PrestConf config variable
var PrestConf *Prest

func viperCfg() {
	filePath := getDefaultPrestConf(os.Getenv("PREST_CONF"))
	dir, file := filepath.Split(filePath)
	file = strings.TrimSuffix(file, filepath.Ext(file))
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvPrefix("PREST")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(replacer)
	viper.AddConfigPath(dir)
	viper.SetConfigName(file)
	viper.SetConfigType("toml")
	viper.SetDefault("http.port", 3000)
	viper.SetDefault("pg.host", "127.0.0.1")
	viper.SetDefault("pg.port", 5432)
	viper.SetDefault("pg.maxidleconn", 10)
	viper.SetDefault("pg.maxopenconn", 10)
	viper.SetDefault("pg.conntimeout", 10)
	viper.SetDefault("debug", false)
	viper.SetDefault("cors.allowheaders", []string{"*"})

	user, err := user.Current()
	if err != nil {
		log.Println("{viperCfg}", err)
	}

	viper.SetDefault("queries.location", filepath.Join(user.HomeDir, "queries"))
}

func getDefaultPrestConf(prestConf string) string {
	if prestConf == "" {
		return "./prest.toml"
	}
	return prestConf
}

// Parse pREST config
func Parse(cfg *Prest) (err error) {
	err = viper.ReadInConfig()
	if err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			log.Warningln("Running without config file.")
		default:
			return
		}
	}
	cfg.HTTPPort = viper.GetInt("http.port")
	cfg.PGHost = viper.GetString("pg.host")
	cfg.PGPort = viper.GetInt("pg.port")
	cfg.PGUser = viper.GetString("pg.user")
	cfg.PGPass = viper.GetString("pg.pass")
	cfg.PGDatabase = viper.GetString("pg.database")
	cfg.PGMaxIdleConn = viper.GetInt("pg.maxidleconn")
	cfg.PGMAxOpenConn = viper.GetInt("pg.maxopenconn")
	cfg.PGConnTimeout = viper.GetInt("pg.conntimeout")
	cfg.JWTKey = viper.GetString("jwt.key")
	cfg.MigrationsPath = viper.GetString("migrations")
	cfg.AccessConf.Restrict = viper.GetBool("access.restrict")
	cfg.QueriesPath = viper.GetString("queries.location")
	cfg.CORSAllowOrigin = viper.GetStringSlice("cors.alloworigin")
	cfg.CORSAllowHeaders = viper.GetStringSlice("cors.allowheaders")
	cfg.Debug = viper.GetBool("debug")

	var t []TablesConf
	err = viper.UnmarshalKey("access.tables", &t)
	if err != nil {
		return err
	}

	cfg.AccessConf.Tables = t

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

	if !PrestConf.AccessConf.Restrict {
		log.Warningln("You are running pREST in public mode.")
	}

	if PrestConf.Debug {
		log.DebugMode = PrestConf.Debug
		log.Warningln("You are running pREST in debug mode.")
	}

	if _, err = os.Stat(PrestConf.QueriesPath); os.IsNotExist(err) {
		if err = os.MkdirAll(PrestConf.QueriesPath, 0700); os.IsNotExist(err) {
			log.Errorf("Queries directory %s is not created", PrestConf.QueriesPath)
		}
	}
}

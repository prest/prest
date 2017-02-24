package config

import (
	"fmt"
	"log"
	"os/user"
	"path/filepath"
	"strings"

	"os"

	"github.com/spf13/viper"
)

type TablesConf struct {
	Name        string   `mapstructure:"name"`
	Permissions []string `mapstructure:"permissions"`
	Fields      []string `mapstructure:"fields"`
}

type AccessConf struct {
	Restrict bool
	Tables   []TablesConf
}

// Prest basic config
type Prest struct {
	// HTTPPort Declare which http port the PREST used
	HTTPPort        int
	PGHost          string
	PGPort          int
	PGUser          string
	PGPass          string
	PGDatabase      string
	PGMaxIdleConn   int
	PGMAxOpenConn   int
	JWTKey          string
	MigrationsPath  string
	QueriesPath     string
	AccessConf      AccessConf
	CORSAllowOrigin []string
}

var PREST_CONF *Prest

func init() {
	viperCfg()
}

func viperCfg() {
	filePath := os.Getenv("PREST_CONF")
	if filePath == "" {
		filePath = "prest.toml"
	}
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvPrefix("PREST")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigFile(filePath)
	viper.SetConfigType("toml")
	viper.SetDefault("http.port", 3000)
	viper.SetDefault("pg.host", "127.0.0.1")
	viper.SetDefault("pg.port", 5432)
	viper.SetDefault("pg.maxidleconn", 10)
	viper.SetDefault("pg.maxopenconn", 10)

	user, err := user.Current()
	if err != nil {
		log.Println("{viperCfg}", err)
	}

	viper.SetDefault("queries.location", filepath.Join(user.HomeDir, "queries"))
}

// Parse pREST config
func Parse(cfg *Prest) (err error) {
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	cfg.HTTPPort = viper.GetInt("http.port")
	cfg.PGHost = viper.GetString("pg.host")
	cfg.PGPort = viper.GetInt("pg.port")
	cfg.PGUser = viper.GetString("pg.user")
	cfg.PGPass = viper.GetString("pg.pass")
	cfg.PGDatabase = viper.GetString("pg.database")
	cfg.PGMaxIdleConn = viper.GetInt("pg.maxidleconn")
	cfg.PGMAxOpenConn = viper.GetInt("pg.maxopenconn")
	cfg.JWTKey = viper.GetString("jwt.key")
	cfg.MigrationsPath = viper.GetString("migrations")
	cfg.AccessConf.Restrict = viper.GetBool("access.restrict")
	cfg.QueriesPath = viper.GetString("queries.location")
	cfg.CORSAllowOrigin = viper.GetStringSlice("cors.alloworigin")

	var t []TablesConf
	err = viper.UnmarshalKey("access.tables", &t)
	if err != nil {
		return err
	}

	cfg.AccessConf.Tables = t

	return
}

func InitConf() {
	viperCfg()
	prestConfig := Prest{}
	Parse(&prestConfig)
	PREST_CONF = &prestConfig

	if !prestConfig.AccessConf.Restrict {
		fmt.Println("You are running pREST in public mode.")
	}

	if _, err := os.Stat(PREST_CONF.QueriesPath); os.IsNotExist(err) {
		os.MkdirAll(PREST_CONF.QueriesPath, 0777)
	}
}

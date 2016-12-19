package config

import (
	"strings"

	"os"

	"github.com/spf13/viper"
)

// Prest basic config
type Prest struct {
	// HTTPPort Declare which http port the PREST used
	HTTPPort   int
	PGHost     string
	PGPort     int
	PGUser     string
	PGPass     string
	PGDatabase string
	JWTKey     string
}

func init() {
	viperCfg()
}

func viperCfg() {
	filePath := os.Getenv("PREST_CONF")
	if filePath == "" {
		filePath = "prest.json"
	}
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvPrefix("PREST")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigFile(filePath)
	viper.SetConfigType("json")
	viper.SetDefault("http.port", 3000)
	viper.SetDefault("pg.host", "127.0.0.1")
	viper.SetDefault("pg.port", 5432)
}

// Parse pREST config
func Parse(cfg *Prest) (err error) {
	err = viper.ReadInConfig()
	cfg.HTTPPort = viper.GetInt("http.port")
	cfg.PGHost = viper.GetString("pg.host")
	cfg.PGPort = viper.GetInt("pg.port")
	cfg.PGUser = viper.GetString("pg.user")
	cfg.PGPass = viper.GetString("pg.pass")
	cfg.PGDatabase = viper.GetString("pg.database")
	cfg.JWTKey = viper.GetString("jwt.key")
	return
}

package config

import "github.com/spf13/viper"

func setAuthDefaults(v *viper.Viper) {
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.username", "username")
	v.SetDefault("auth.password", "password")
	v.SetDefault("auth.schema", "public")
	v.SetDefault("auth.table", "prest_users")
	v.SetDefault("auth.encrypt", "bcrypt")
	v.SetDefault("auth.type", "body")
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

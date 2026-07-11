package config

import (
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

const (
	QueriesStorageFilesystem = "filesystem"
	QueriesStorageDatabase   = "database"

	QueriesImportPolicySkip   = "skip"
	QueriesImportPolicyUpdate = "update"
	QueriesImportPolicyError  = "error"
)

// ScriptConf holds ACL rules for a custom query script.
type ScriptConf struct {
	Database    string   `mapstructure:"database"`
	Location    string   `mapstructure:"location"`
	Name        string   `mapstructure:"name"`
	Permissions []string `mapstructure:"permissions"`
}

// QueryUsersConf holds per-user script ACL overrides.
type QueryUsersConf struct {
	Name    string       `mapstructure:"name"`
	Scripts []ScriptConf `mapstructure:"scripts"`
}

// QueriesConf holds custom query storage and access settings.
type QueriesConf struct {
	Storage          string
	Schema           string
	Table            string
	Restrict         bool
	RegisterEnabled  bool
	RegisterAdmins   []string
	MigrateOnStartup bool
	ImportOnStartup  bool
	ImportPolicy     string
	Scripts          []ScriptConf
	Users            []QueryUsersConf
}

func parseQueriesConfig(v *viper.Viper, cfg *Prest) {
	q := &cfg.QueriesConf
	q.Storage = strings.ToLower(strings.TrimSpace(v.GetString("queries.storage")))
	if q.Storage == "" {
		q.Storage = QueriesStorageFilesystem
	}
	if q.Storage != QueriesStorageFilesystem && q.Storage != QueriesStorageDatabase {
		slog.Warn("queries.storage invalid, using filesystem", "value", q.Storage)
		q.Storage = QueriesStorageFilesystem
	}

	q.Schema = v.GetString("queries.schema")
	if q.Schema == "" {
		q.Schema = "public"
	}
	q.Table = v.GetString("queries.table")
	if q.Table == "" {
		q.Table = "prest_queries"
	}

	q.Restrict = v.GetBool("queries.restrict")
	q.RegisterEnabled = v.GetBool("queries.register_enabled")
	q.RegisterAdmins = v.GetStringSlice("queries.register_admins")

	if v.IsSet("queries.migrate_on_startup") {
		q.MigrateOnStartup = v.GetBool("queries.migrate_on_startup")
	} else {
		q.MigrateOnStartup = q.Storage == QueriesStorageDatabase
	}

	if v.IsSet("queries.import_on_startup") {
		q.ImportOnStartup = v.GetBool("queries.import_on_startup")
	} else {
		q.ImportOnStartup = q.Storage == QueriesStorageDatabase
	}

	q.ImportPolicy = strings.ToLower(strings.TrimSpace(v.GetString("queries.import_policy")))
	if q.ImportPolicy == "" {
		q.ImportPolicy = QueriesImportPolicyUpdate
	}

	if err := v.UnmarshalKey("queries.scripts", &q.Scripts); err != nil {
		slog.Warn("config key invalid, using default", "key", "queries.scripts", "err", err)
		q.Scripts = nil
	}
	if err := v.UnmarshalKey("queries.users", &q.Users); err != nil {
		slog.Warn("config key invalid, using default", "key", "queries.users", "err", err)
		q.Users = nil
	}
}

func ensureQueriesConfig(cfg *Prest) {
	q := &cfg.QueriesConf

	if q.RegisterEnabled {
		if !cfg.AuthEnabled || cfg.JWTKey == "" {
			slog.Error("query registration disabled: auth.enabled and jwt.key required",
				"err", ErrQueryRegisterAuthRequired)
			q.RegisterEnabled = false
		} else if len(q.RegisterAdmins) == 0 {
			slog.Error("query registration disabled: queries.register_admins is empty",
				"err", ErrQueryRegisterNoAdmins)
			q.RegisterEnabled = false
		}
	}

	if q.Restrict && !cfg.AuthEnabled {
		slog.Error("query ACL disabled: queries.restrict requires auth.enabled",
			"err", ErrQueryRestrictAuthRequired)
		q.Restrict = false
	}

	if q.ImportPolicy != QueriesImportPolicySkip &&
		q.ImportPolicy != QueriesImportPolicyUpdate &&
		q.ImportPolicy != QueriesImportPolicyError {
		slog.Warn("queries.import_policy invalid, using update", "value", q.ImportPolicy)
		q.ImportPolicy = QueriesImportPolicyUpdate
	}
}

// ErrQueryRegisterAuthRequired is returned when query registration is enabled without auth.
var ErrQueryRegisterAuthRequired = errQueryRegisterAuthRequired{}

type errQueryRegisterAuthRequired struct{}

func (errQueryRegisterAuthRequired) Error() string {
	return "queries.register_enabled requires auth.enabled and jwt.key"
}

// ErrQueryRegisterNoAdmins is returned when query registration has no admin allow-list.
var ErrQueryRegisterNoAdmins = errQueryRegisterNoAdmins{}

type errQueryRegisterNoAdmins struct{}

func (errQueryRegisterNoAdmins) Error() string {
	return "queries.register_enabled requires queries.register_admins"
}

// ErrQueryRestrictAuthRequired is returned when query ACL is enabled without auth.
var ErrQueryRestrictAuthRequired = errQueryRestrictAuthRequired{}

type errQueryRestrictAuthRequired struct{}

func (errQueryRestrictAuthRequired) Error() string {
	return "queries.restrict requires auth.enabled"
}

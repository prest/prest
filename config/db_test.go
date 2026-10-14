package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func Test_parseDatabaseURL(t *testing.T) {
	t.Parallel()

	t.Run("valid URL with sslmode", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user:pass@localhost:5432/mydatabase/?sslmode=require"}
		parseDatabaseURL(c)
		require.Equal(t, "mydatabase", c.PGDatabase)
		require.Equal(t, 5432, c.PGPort)
		require.Equal(t, "user", c.PGUser)
		require.Equal(t, "pass", c.PGPass)
		require.Equal(t, "require", c.PGSSLMode)
	})

	t.Run("empty URL is a no-op", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGHost: "keep", PGDatabase: "keep"}
		parseDatabaseURL(c)
		require.Equal(t, "keep", c.PGHost)
		require.Equal(t, "keep", c.PGDatabase)
	})

	t.Run("invalid port aborts URL parsing", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user:pass@localhost:999999999999999999999/mydatabase/?sslmode=require"}
		parseDatabaseURL(c)
		require.Equal(t, "localhost", c.PGHost)
		require.Empty(t, c.PGDatabase)
	})

	t.Run("invalid URL", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: `invalid%+o`}
		parseDatabaseURL(c)
		require.Equal(t, "", c.PGDatabase)
		require.Equal(t, "", c.PGUser)
	})

	t.Run("URL without password", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user@localhost/mydatabase"}
		parseDatabaseURL(c)
		require.Equal(t, "mydatabase", c.PGDatabase)
		require.Equal(t, "user", c.PGUser)
		require.Empty(t, c.PGPass)
	})
}

// pg.allowed_databases comes from a TOML array; blank and padded items are
// normalised so IsRegistered compares exact names.
func TestParseDBConfig_AllowedDatabasesFromTOML(t *testing.T) {
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`
[pg]
database = "prest"
allowed_databases = ["analytics", " reports ", ""]
`)))
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	require.Equal(t, []string{"analytics", "reports"}, cfg.AllowedDatabases)
}

// PREST_PG_ALLOWED_DATABASES is comma separated; items are trimmed and empty
// entries dropped. The env value overrides the file.
func TestParseDBConfig_AllowedDatabasesFromEnv(t *testing.T) {
	t.Setenv("PREST_PG_ALLOWED_DATABASES", " analytics, ,reports,")
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`
[pg]
allowed_databases = ["ignored"]
`)))
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	require.Equal(t, []string{"analytics", "reports"}, cfg.AllowedDatabases)
}

// A TOML string value is accepted as a comma-separated list too.
func TestParseDBConfig_AllowedDatabasesTOMLString(t *testing.T) {
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`
[pg]
allowed_databases = "a, b"
`)))
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	require.Equal(t, []string{"a", "b"}, cfg.AllowedDatabases)
}

// Unset means no extra databases: only pg.database is reachable.
func TestParseDBConfig_AllowedDatabasesUnset(t *testing.T) {
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(viper.New(), cfg))
	require.Empty(t, cfg.AllowedDatabases)
}

// The MySQL engine reads the same pg.allowed_databases key.
func TestParseDBConfig_AllowedDatabasesMySQL(t *testing.T) {
	t.Setenv("PREST_PG_ALLOWED_DATABASES", "shop")
	v := viper.New()
	v.Set("engine", "mysql")
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	require.Equal(t, []string{"shop"}, cfg.AllowedDatabases)
}

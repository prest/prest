package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	Load()
	require.Greaterf(t, len(PrestConf.AccessConf.Tables), 2,
		"expected > 2, got: %d", len(PrestConf.AccessConf.Tables))

	for _, ignoretable := range PrestConf.AccessConf.IgnoreTable {
		require.Equal(t, "test_permission_does_not_exist", ignoretable,
			"expected ['test_permission_does_not_exist'], but got another result")
	}
	require.True(t, PrestConf.AccessConf.Restrict, "expected true, but got false")
	require.Equal(t, 60, PrestConf.HTTPTimeout)

	os.Setenv("PREST_CONF", "foo/bar/prest.toml")
}

func TestParse(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 3000, cfg.HTTPPort)

	var expected string
	expected = os.Getenv("PREST_PG_DATABASE")
	if len(expected) == 0 {
		expected = "prest"
	}

	require.Equal(t, expected, cfg.PGDatabase)

	os.Unsetenv("PREST_CONF")
	os.Unsetenv("PREST_JWT_DEFAULT")
	os.Setenv("PREST_HTTP_PORT", "4000")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 4000, cfg.HTTPPort)
	require.True(t, cfg.EnableDefaultJWT)

	os.Unsetenv("PREST_CONF")

	os.Setenv("PREST_CONF", "")
	os.Setenv("PREST_JWT_DEFAULT", "false")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 4000, cfg.HTTPPort)
	require.False(t, cfg.EnableDefaultJWT)

	os.Unsetenv("PREST_JWT_DEFAULT")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 4000, cfg.HTTPPort)

	os.Unsetenv("PREST_CONF")
	os.Unsetenv("PREST_HTTP_PORT")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, "s3cr3t", cfg.JWTKey)
	require.Equal(t, "HS256", cfg.JWTAlgo)

	os.Unsetenv("PREST_JWT_KEY")
	os.Setenv("PREST_JWT_ALGO", "HS512")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, "HS512", cfg.JWTAlgo)

	os.Unsetenv("PREST_JWT_ALGO")
}

func TestGetDefaultPrestConf(t *testing.T) {
	testCases := []struct {
		name      string
		prestConf string
		result    string
	}{
		{"custom config", "../prest.toml", "../prest.toml"},
		{"default config", "", "./prest.toml"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getDefaultPrestConf(tc.prestConf)
			if tc.prestConf == "" {
				if cfg != "./prest.toml" {
					t.Errorf("expected %v, but got %v", tc.result, cfg)
				}
			} else {
				if cfg != tc.result || cfg == "./prest.toml" {
					t.Errorf("expected %v, but got %v", tc.result, cfg)
				}
			}

		})
	}
}

func TestDatabaseURL(t *testing.T) {
	os.Setenv("PREST_PG_URL", "postgresql://user:pass@localhost:1234/mydatabase/?sslmode=disable")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, "mydatabase", cfg.PGDatabase)
	require.Equal(t, "localhost", cfg.PGHost)
	require.Equal(t, 1234, cfg.PGPort)
	require.Equal(t, "user", cfg.PGUser)
	require.Equal(t, "pass", cfg.PGPass)
	require.Equal(t, "disable", cfg.SSLMode)

	os.Unsetenv("PREST_PG_URL")
	os.Setenv("DATABASE_URL", "postgresql://cloud:cloudPass@localhost:5432/CloudDatabase/?sslmode=disable")

	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 5432, cfg.PGPort)
	require.Equal(t, "cloud", cfg.PGUser)
	require.Equal(t, "cloudPass", cfg.PGPass)
	require.Equal(t, "disable", cfg.SSLMode)

	os.Unsetenv("DATABASE_URL")
}

func TestHTTPPort(t *testing.T) {
	os.Setenv("PORT", "8080")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.HTTPPort)

	// set env PREST_HTTP_PORT and PORT
	os.Setenv("PREST_HTTP_PORT", "3000")

	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.HTTPPort)

	// unset env PORT and set PREST_HTTP_PORT
	os.Unsetenv("PORT")

	cfg = &Prest{}
	err = Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, 3000, cfg.HTTPPort)

	os.Unsetenv("PREST_HTTP_PORT")
}

func Test_parseDatabaseURL(t *testing.T) {
	c := &Prest{PGURL: "postgresql://user:pass@localhost:5432/mydatabase/?sslmode=require"}
	err := parseDatabaseURL(c)
	require.NoError(t, err)
	require.Equal(t, "mydatabase", c.PGDatabase)
	require.Equal(t, 5432, c.PGPort)
	require.Equal(t, "user", c.PGUser)
	require.Equal(t, "pass", c.PGPass)
	require.Equal(t, "require", c.SSLMode)

	// errors
	c = &Prest{PGURL: "postgresql://user:pass@localhost:port/mydatabase/?sslmode=require"}
	err = parseDatabaseURL(c)
	require.Error(t, err)
}

func Test_portFromEnv(t *testing.T) {
	c := &Prest{}

	os.Setenv("PORT", "PORT")

	err := portFromEnv(c)
	require.Error(t, err)

	os.Unsetenv("PORT")
}

func Test_Auth(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, false, cfg.AuthEnabled)
	require.Equal(t, "public", cfg.AuthSchema)
	require.Equal(t, "prest_users", cfg.AuthTable)
	require.Equal(t, "username", cfg.AuthUsername)
	require.Equal(t, "password", cfg.AuthPassword)
	require.Equal(t, "MD5", cfg.AuthEncrypt)

	metadata := []string{"first_name", "last_name", "last_login"}
	require.Equal(t, len(metadata), len(cfg.AuthMetadata))

	for i, v := range cfg.AuthMetadata {
		require.Equal(t, metadata[i], v)
	}
}

func Test_ExposeDataConfig(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest_expose.toml")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	require.NoError(t, err)
	require.Equal(t, true, cfg.ExposeConf.Enabled)
	require.Equal(t, false, cfg.ExposeConf.DatabaseListing)
	require.Equal(t, false, cfg.ExposeConf.SchemaListing)
	require.Equal(t, false, cfg.ExposeConf.TableListing)

	metadata := []string{"first_name", "last_name", "last_login"}
	require.Equal(t, len(metadata), len(cfg.AuthMetadata))

	for i, v := range cfg.AuthMetadata {
		require.Equal(t, metadata[i], v)
	}
}

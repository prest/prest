package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	Load()
	if len(PrestConf.AccessConf.Tables) < 2 {
		t.Errorf("expected > 2, got: %d", len(PrestConf.AccessConf.Tables))
	}

	Load()
	for _, ignoretable := range PrestConf.AccessConf.IgnoreTable {
		if ignoretable != "test_permission_does_not_exist" {
			t.Error("expected ['test_permission_does_not_exist'], but got another result")
		}
	}

	Load()
	if !PrestConf.AccessConf.Restrict {
		t.Error("expected true, but got false")
	}

	os.Setenv("PREST_CONF", "foo/bar/prest.toml")
	Load()
}

func TestParse(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 6000 {
		t.Errorf("expected port: 6000, got: %d", cfg.HTTPPort)
	}

	var expected string
	expected = os.Getenv("PREST_PG_DATABASE")
	if len(expected) == 0 {
		expected = "prest"
	}
	if cfg.PGDatabase != expected {
		t.Errorf("expected database: %s, got: %s", expected, cfg.PGDatabase)
	}

	os.Unsetenv("PREST_CONF")
	os.Unsetenv("PREST_JWT_DEFAULT")
	os.Setenv("PREST_HTTP_PORT", "4000")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}
	if !cfg.EnableDefaultJWT {
		t.Error("EnableDefaultJWT: expected true but got false")
	}

	os.Unsetenv("PREST_CONF")

	os.Setenv("PREST_CONF", "")
	os.Setenv("PREST_JWT_DEFAULT", "false")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}
	if cfg.EnableDefaultJWT {
		t.Error("EnableDefaultJWT: expected false but got true")
	}

	os.Unsetenv("PREST_JWT_DEFAULT")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}

	os.Unsetenv("PREST_CONF")
	os.Unsetenv("PREST_HTTP_PORT")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.JWTKey != "s3cr3t" {
		t.Errorf("expected jwt key: s3cr3t, got: %s", cfg.JWTKey)
	}
	if cfg.JWTAlgo != "HS256" {
		t.Errorf("expected (default) jwt algo: HS256, got: %s", cfg.JWTAlgo)
	}

	os.Unsetenv("PREST_JWT_KEY")
	os.Setenv("PREST_JWT_ALGO", "HS512")

	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.JWTAlgo != "HS512" {
		t.Errorf("expected jwt algo: HS512, got: %s", cfg.JWTAlgo)
	}

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
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.PGDatabase != "mydatabase" {
		t.Errorf("expected database name: mydatabase, got: %s", cfg.PGDatabase)
	}
	if cfg.PGHost != "localhost" {
		t.Errorf("expected database host: localhost, got: %s", cfg.PGHost)
	}
	if cfg.PGPort != 1234 {
		t.Errorf("expected database port: 1234, got: %d", cfg.PGPort)
	}
	if cfg.PGUser != "user" {
		t.Errorf("expected database user: user, got: %s", cfg.PGUser)
	}
	if cfg.PGPass != "pass" {
		t.Errorf("expected database password: pass, got: %s", cfg.PGPass)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected database ssl mode: disable, got: %s", cfg.SSLMode)
	}

	os.Unsetenv("PREST_PG_URL")
	os.Setenv("DATABASE_URL", "postgresql://cloud:cloudPass@localhost:5432/CloudDatabase/?sslmode=disable")

	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.PGPort != 5432 {
		t.Errorf("expected database port: 5432, got: %d", cfg.PGPort)
	}
	if cfg.PGUser != "cloud" {
		t.Errorf("expected database user: cloud, got: %s", cfg.PGUser)
	}
	if cfg.PGPass != "cloudPass" {
		t.Errorf("expected database password: cloudPass, got: %s", cfg.PGPass)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("expected database SSL mode: disable, got: %s", cfg.SSLMode)
	}

	os.Unsetenv("DATABASE_URL")
}

func TestHTTPPort(t *testing.T) {
	os.Setenv("PORT", "8080")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected http port: 8080, got: %d", cfg.HTTPPort)
	}

	// set env PREST_HTTP_PORT and PORT
	os.Setenv("PREST_HTTP_PORT", "3000")

	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected http port: 8080, got: %d", cfg.HTTPPort)
	}

	// unset env PORT and set PREST_HTTP_PORT
	os.Unsetenv("PORT")

	cfg = &Prest{}
	err = Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if cfg.HTTPPort != 3000 {
		t.Errorf("expected http port: 3000, got: %d", cfg.HTTPPort)
	}

	os.Unsetenv("PREST_HTTP_PORT")
}

func Test_parseDatabaseURL(t *testing.T) {
	c := &Prest{PGURL: "postgresql://user:pass@localhost:5432/mydatabase/?sslmode=require"}
	if err := parseDatabaseURL(c); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if c.PGDatabase != "mydatabase" {
		t.Errorf("expected database name: mydatabase, got: %s", c.PGDatabase)
	}
	if c.PGPort != 5432 {
		t.Errorf("expected database port: 5432, got: %d", c.PGPort)
	}
	if c.PGUser != "user" {
		t.Errorf("expected database user: user, got: %s", c.PGUser)
	}
	if c.PGPass != "pass" {
		t.Errorf("expected database password: password, got: %s", c.PGPass)
	}
	if c.SSLMode != "require" {
		t.Errorf("expected database SSL mode: require, got: %s", c.SSLMode)
	}

	// errors
	c = &Prest{PGURL: "postgresql://user:pass@localhost:port/mydatabase/?sslmode=require"}
	if err := parseDatabaseURL(c); err == nil {
		t.Error("expected error, got nothing")
	}
}

func Test_portFromEnv(t *testing.T) {
	c := &Prest{}

	os.Setenv("PORT", "PORT")

	err := portFromEnv(c)
	if err == nil {
		t.Errorf("expect error, got: %d", c.HTTPPort)
	}

	os.Unsetenv("PORT")
}

func Test_Auth(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")

	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.AuthEnabled != false {
		t.Errorf("expected auth.enabled to be: false, got: %v", cfg.AuthEnabled)
	}

	if cfg.AuthSchema != "public" {
		t.Errorf("expected auth.schema to be: public, got: %s", cfg.AuthSchema)
	}

	if cfg.AuthTable != "prest_users" {
		t.Errorf("expected auth.table to be: prest_users, got: %s", cfg.AuthTable)
	}

	if cfg.AuthUsername != "username" {
		t.Errorf("expected auth.username to be: username, got: %s", cfg.AuthUsername)
	}

	if cfg.AuthPassword != "password" {
		t.Errorf("expected auth.password to be: password, got: %s", cfg.AuthPassword)
	}

	if cfg.AuthEncrypt != "MD5" {
		t.Errorf("expected auth.encrypt to be: MD5, got: %s", cfg.AuthEncrypt)
	}

	metadata := []string{"first_name", "last_name", "last_login"}
	if len(cfg.AuthMetadata) != len(metadata) {
		t.Errorf("expected auth.metadata to be: %d, got: %d", len(metadata), len(cfg.AuthMetadata))
	}

	for i, v := range cfg.AuthMetadata {
		if v != metadata[i] {
			t.Errorf("expected auth.metadata field %d to be: %s, got: %s", i, metadata[i], v)
		}
	}
}

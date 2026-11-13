package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAllowedHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		entry string
		want  string
	}{
		{entry: "example.com", want: "example.com"},
		{entry: "  API.Example.COM  ", want: "api.example.com"},
		{entry: "example.com.", want: "example.com"},
		{entry: ".example.com", want: ".example.com"},
		{entry: ".Example.com.", want: ".example.com"},
		{entry: "*", want: "*"},
		{entry: "127.0.0.1", want: "127.0.0.1"},
		{entry: "::1", want: "[::1]"},
		{entry: "[::1]", want: "[::1]"},
		{entry: "2001:DB8::1", want: "[2001:db8::1]"},
		{entry: "example.com:3000", want: "example.com"},
		{entry: "127.0.0.1:3000", want: "127.0.0.1"},
		{entry: "[::1]:3000", want: "[::1]"},
		{entry: "", want: ""},
		{entry: "   ", want: ""},
		{entry: ".", want: ""},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, normalizeAllowedHost(tc.entry), "entry %q", tc.entry)
	}
}

func TestParseAllowedHosts(t *testing.T) {
	t.Run("unset serves every host", func(t *testing.T) {
		t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
		unsetEnvForTest(t, "PREST_HTTP_ALLOWED_HOSTS")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Nil(t, cfg.HTTPAllowedHosts)
	})

	t.Run("config file array", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nallowed_hosts = [\"API.example.com\", \".example.org\", \"\", \"[::1]\"]\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		unsetEnvForTest(t, "PREST_HTTP_ALLOWED_HOSTS")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, []string{"api.example.com", ".example.org", "[::1]"}, cfg.HTTPAllowedHosts)
	})

	t.Run("config file comma-separated string", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nallowed_hosts = \"example.com, localhost\"\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		unsetEnvForTest(t, "PREST_HTTP_ALLOWED_HOSTS")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, []string{"example.com", "localhost"}, cfg.HTTPAllowedHosts)
	})

	t.Run("PREST_HTTP_ALLOWED_HOSTS overrides the config file", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nallowed_hosts = [\"file.example.com\"]\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		t.Setenv("PREST_HTTP_ALLOWED_HOSTS", "api.example.com:443, .example.org,,localhost")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, []string{"api.example.com", ".example.org", "localhost"}, cfg.HTTPAllowedHosts)
	})

	// viper ignores an empty environment variable, so an empty
	// PREST_HTTP_ALLOWED_HOSTS cannot switch off the list in the config file.
	t.Run("empty PREST_HTTP_ALLOWED_HOSTS keeps the config file list", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nallowed_hosts = [\"file.example.com\"]\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		t.Setenv("PREST_HTTP_ALLOWED_HOSTS", "")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, []string{"file.example.com"}, cfg.HTTPAllowedHosts)
	})

	// A value with entries but none usable must not read as "unset".
	t.Run("only unusable entries serve no host", func(t *testing.T) {
		t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
		t.Setenv("PREST_HTTP_ALLOWED_HOSTS", ". , :8080")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.NotNil(t, cfg.HTTPAllowedHosts)
		require.Empty(t, cfg.HTTPAllowedHosts)
	})
}

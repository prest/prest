package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHeaderFieldName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"X-Request-Id", "x-correlation-id", "Request_ID", "a", "X-Trace!#$%&'*+.^_`|~1"} {
		require.True(t, isHeaderFieldName(name), "name %q", name)
	}
	for _, name := range []string{"", "X Request Id", "X-Request-Id:", "X-Request-Id\n", "(id)", "X-Request-Ídentifier"} {
		require.False(t, isHeaderFieldName(name), "name %q", name)
	}
}

func TestParseRequestIDHeader(t *testing.T) {
	t.Run("unset disables request IDs", func(t *testing.T) {
		t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
		unsetEnvForTest(t, "PREST_HTTP_REQUEST_ID_HEADER")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Empty(t, cfg.HTTPRequestIDHeader)
	})

	t.Run("config file value is canonicalized", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nrequest_id_header = \" x-request-id \"\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		unsetEnvForTest(t, "PREST_HTTP_REQUEST_ID_HEADER")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, "X-Request-Id", cfg.HTTPRequestIDHeader)
	})

	t.Run("PREST_HTTP_REQUEST_ID_HEADER overrides the config file", func(t *testing.T) {
		conf := filepath.Join(t.TempDir(), "prest.toml")
		require.NoError(t, os.WriteFile(conf, []byte(
			"[http]\nrequest_id_header = \"X-Request-Id\"\n"), 0600))
		t.Setenv("PREST_CONF", conf)
		t.Setenv("PREST_HTTP_REQUEST_ID_HEADER", "X-Correlation-Id")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Equal(t, "X-Correlation-Id", cfg.HTTPRequestIDHeader)
	})

	t.Run("invalid header name disables request IDs", func(t *testing.T) {
		t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
		t.Setenv("PREST_HTTP_REQUEST_ID_HEADER", "X Request Id")
		v, configPath := viperCfg()
		cfg := &Prest{}
		requireParse(t, v, cfg, configPath)
		require.Empty(t, cfg.HTTPRequestIDHeader)
	})
}

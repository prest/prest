package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// runExportJSON runs `config export-json` with flags and returns its stdout.
func runExportJSON(t *testing.T, flags map[string]string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	configExportJSONCmd.SetOut(&out)
	t.Cleanup(func() {
		configExportJSONCmd.SetOut(nil)
		exportJSONFile, exportJSONPretty, exportJSONIncludeInfra = "", false, false
	})
	for name, value := range flags {
		require.NoError(t, configExportJSONCmd.Flags().Set(name, value))
	}
	err := configExportJSONCmd.RunE(configExportJSONCmd, nil)
	return out.String(), err
}

func TestConfigExportJSON_Stdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prest.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
debug = true

[http]
port = 4000

[access]
restrict = true
`), 0600))

	out, err := runExportJSON(t, map[string]string{"file": path})
	require.NoError(t, err)
	require.True(t, json.Valid([]byte(out)), out)
	require.JSONEq(t, `{"debug": true, "access": {"restrict": true}}`, out)

	out, err = runExportJSON(t, map[string]string{"file": path, "include-infra": "true", "pretty": "true"})
	require.NoError(t, err)
	require.JSONEq(t, `{"debug": true, "access": {"restrict": true}, "http": {"port": 4000}}`, out)
	require.Contains(t, out, "\n  \"access\": {")
}

func TestConfigExportJSON_BadFile(t *testing.T) {
	out, err := runExportJSON(t, map[string]string{"file": filepath.Join(t.TempDir(), "missing.toml")})
	require.Error(t, err)
	require.Empty(t, out)
}

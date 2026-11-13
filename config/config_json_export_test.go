package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTOML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prest.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func exportedDoc(t *testing.T, path string, includeInfra bool) map[string]any {
	t.Helper()
	out, err := ExportConfigJSON(path, false, includeInfra)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(out, &doc))
	return doc
}

func TestExportJSON_StripsInfra(t *testing.T) {
	path := writeTOML(t, configJSONTestTOML+`
engine = "postgres"

[https]
mode = false

[mysql]
prepare = true

[queries]
location = "/srv/queries"
restrict = true

[[databases]]
alias = "tenant"
url = "postgres://db.internal/tenant"
`)
	doc := exportedDoc(t, path, false)
	for _, key := range []string{"http", "https", "engine", "pg", "mysql", "databases"} {
		require.NotContains(t, doc, key)
	}
	require.Equal(t, map[string]any{"restrict": true}, doc["queries"])
	require.Equal(t, false, doc["debug"])
	require.Equal(t, "/api", doc["context"])
	require.Len(t, doc["access"].(map[string]any)["tables"], 2)
}

// A section holding only infra keys is dropped whole, not exported as {}.
func TestExportJSON_DropsEmptiedTables(t *testing.T) {
	path := writeTOML(t, `
debug = true

[queries]
location = "/srv/queries"
`)
	require.Equal(t, map[string]any{"debug": true}, exportedDoc(t, path, false))
}

func TestExportJSON_IncludeInfra(t *testing.T) {
	doc := exportedDoc(t, writeTOML(t, configJSONTestTOML), true)
	require.Equal(t, map[string]any{"port": float64(4000)}, doc["http"])
	require.Equal(t, map[string]any{"host": "db.internal"}, doc["pg"])
}

// Defaults and env vars are not baked into the export: only the file's keys.
func TestExportJSON_FileKeysOnly(t *testing.T) {
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_AUTH_ENABLED", "true")
	t.Setenv(EnvConfigJSON, `{"cors": {"alloworigin": ["https://json.example"]}}`)
	doc := exportedDoc(t, writeTOML(t, "debug = false\n"), false)
	require.Equal(t, map[string]any{"debug": false}, doc)
}

func TestExportJSON_Pretty(t *testing.T) {
	path := writeTOML(t, "debug = true\n")
	out, err := ExportConfigJSON(path, true, false)
	require.NoError(t, err)
	require.Equal(t, "{\n  \"debug\": true\n}", string(out))

	out, err = ExportConfigJSON(path, false, false)
	require.NoError(t, err)
	require.Equal(t, `{"debug":true}`, string(out))
}

func TestExportJSON_DefaultPath(t *testing.T) {
	t.Setenv("PREST_CONF", writeTOML(t, "context = \"/from-env\"\n"))
	out, err := ExportConfigJSON("", false, false)
	require.NoError(t, err)
	require.JSONEq(t, `{"context": "/from-env"}`, string(out))
}

func TestExportJSON_BadFile(t *testing.T) {
	_, err := ExportConfigJSON(filepath.Join(t.TempDir(), "missing.toml"), false, false)
	require.Error(t, err)

	_, err = ExportConfigJSON(writeTOML(t, "this is not valid [[[toml"), false, false)
	require.Error(t, err)
}

// Loading the export with no prest.toml yields the same config as loading the
// file, apart from the infra settings the export leaves to env vars.
func TestExportJSON_RoundTrip(t *testing.T) {
	for _, file := range []string{"../testdata/prest.toml", "../samples/prest.sample.toml"} {
		t.Run(filepath.Base(file), func(t *testing.T) {
			unsetEnvForTest(t, EnvConfigJSON)
			unsetEnvForTest(t, EnvConfigJSONB64)
			t.Setenv("PREST_CONF", file)
			fromFile, err := Load()
			require.NoError(t, err)

			out, err := ExportConfigJSON(file, false, false)
			require.NoError(t, err)
			t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
			t.Setenv(EnvConfigJSON, string(out))
			fromJSON, err := Load()
			require.NoError(t, err)

			copyInfraConfig(fromJSON, fromFile)
			require.Equal(t, fromFile, fromJSON)
		})
	}
}

// copyInfraConfig copies the settings PREST_CONFIG_JSON never carries, and the
// logger, from src to dst.
func copyInfraConfig(dst, src *Prest) {
	dst.HTTPHost, dst.HTTPPort, dst.HTTPTimeout = src.HTTPHost, src.HTTPPort, src.HTTPTimeout
	dst.HTTPSMode, dst.HTTPSCert, dst.HTTPSKey = src.HTTPSMode, src.HTTPSCert, src.HTTPSKey
	dst.Engine, dst.MySQLPrepare = src.Engine, src.MySQLPrepare
	dst.PGHost, dst.PGPort, dst.PGUser, dst.PGPass = src.PGHost, src.PGPort, src.PGUser, src.PGPass
	dst.PGDatabase, dst.AllowedDatabases, dst.PGURL = src.PGDatabase, src.AllowedDatabases, src.PGURL
	dst.PGSSLMode, dst.PGSSLCert, dst.PGSSLKey, dst.PGSSLRootCert = src.PGSSLMode, src.PGSSLCert, src.PGSSLKey, src.PGSSLRootCert
	dst.PGMaxIdleConn, dst.PGMaxOpenConn, dst.PGConnTimeout, dst.PGCache = src.PGMaxIdleConn, src.PGMaxOpenConn, src.PGConnTimeout, src.PGCache
	dst.Databases, dst.QueriesPath = src.Databases, src.QueriesPath
	dst.Logger = src.Logger
}

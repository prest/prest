package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const configJSONTestTOML = `
debug = false
context = "/api"

[http]
port = 4000

[pg]
host = "db.internal"

[access]
restrict = true

[[access.tables]]
name = "customers"
schema = "public"
permissions = ["read"]

[[access.tables]]
name = "orders"
schema = "public"
permissions = ["read", "write"]

[cors]
alloworigin = ["https://file.example"]
`

// loadWithConfigJSON loads config from a temp prest.toml holding toml (none
// when empty) and a PREST_CONFIG_JSON of doc (unset when empty).
func loadWithConfigJSON(t *testing.T, toml, doc string) (*Prest, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prest.toml")
	if toml != "" {
		require.NoError(t, os.WriteFile(path, []byte(toml), 0600))
	}
	t.Setenv("PREST_CONF", path)
	t.Setenv(EnvConfigJSON, doc)
	unsetEnvForTest(t, EnvConfigJSONB64)
	return Load()
}

func TestApplyConfigJSON_OverridesTOML(t *testing.T) {
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{
		"debug": true,
		"access": {"restrict": false},
		"cors": {"alloworigin": ["https://json.example"]}
	}`)
	require.NoError(t, err)
	require.True(t, cfg.Debug)
	require.False(t, cfg.AccessConf.Restrict)
	require.Equal(t, []string{"https://json.example"}, cfg.CORSAllowOrigin)
	// Keys the document does not name keep their file values.
	require.Equal(t, "/api", cfg.ContextPath)
	require.Len(t, cfg.AccessConf.Tables, 2)
}

func TestApplyConfigJSON_FlatEnvWins(t *testing.T) {
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_CONTEXT", "/env")
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{"debug": true, "context": "/json"}`)
	require.NoError(t, err)
	require.False(t, cfg.Debug)
	require.Equal(t, "/env", cfg.ContextPath)
}

func TestApplyConfigJSON_IgnoresInfraKeys(t *testing.T) {
	t.Setenv("PREST_HTTP_PORT", "5000")
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{
		"http": {"port": 9999, "host": "10.0.0.1"},
		"https": {"mode": true},
		"engine": "mysql",
		"pg": {"host": "evil.example", "database": "other"},
		"mysql": {"prepare": true},
		"databases": [{"alias": "x", "url": "postgres://evil.example/x"}],
		"queries": {"location": "/tmp/elsewhere", "restrict": false},
		"debug": true
	}`)
	require.NoError(t, err)
	require.Equal(t, 5000, cfg.HTTPPort, "infra env wins")
	require.Equal(t, "0.0.0.0", cfg.HTTPHost)
	require.False(t, cfg.HTTPSMode)
	require.Equal(t, EnginePostgres, cfg.Engine)
	require.Equal(t, "db.internal", cfg.PGHost, "file value kept")
	require.Equal(t, "prest", cfg.PGDatabase)
	require.False(t, cfg.MySQLPrepare)
	require.Empty(t, cfg.Databases)
	require.NotEqual(t, "/tmp/elsewhere", cfg.QueriesPath)
	// Application keys next to the ignored ones still apply.
	require.True(t, cfg.Debug)
}

func TestApplyConfigJSON_InfraKeyCaseInsensitive(t *testing.T) {
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{"PG": {"Host": "evil.example"}, "HTTP": {"PORT": 1}}`)
	require.NoError(t, err)
	require.Equal(t, "db.internal", cfg.PGHost)
	require.Equal(t, 4000, cfg.HTTPPort)
}

func TestApplyConfigJSON_ArrayReplace(t *testing.T) {
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{
		"access": {"tables": [
			{"name": "invoices", "schema": "billing", "permissions": ["read"], "fields": ["id"]}
		]}
	}`)
	require.NoError(t, err)
	require.Equal(t, []TablesConf{{
		Schema:      "billing",
		Name:        "invoices",
		Permissions: []string{"read"},
		Fields:      []string{"id"},
	}}, cfg.AccessConf.Tables)
	require.True(t, cfg.AccessConf.Restrict, "sibling file key kept")
}

func TestApplyConfigJSON_NullIsAbsent(t *testing.T) {
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, `{"context": null, "access": {"tables": null}}`)
	require.NoError(t, err)
	require.Equal(t, "/api", cfg.ContextPath)
	require.Len(t, cfg.AccessConf.Tables, 2)
}

func TestApplyConfigJSON_Invalid(t *testing.T) {
	cases := map[string]struct {
		doc  string
		want string
	}{
		"syntax":           {`{"debug": true`, "parse PREST_CONFIG_JSON"},
		"trailing data":    {`{"debug": true} {}`, "parse PREST_CONFIG_JSON"},
		"array root":       {`[{"debug": true}]`, "must be a JSON object"},
		"null root":        {`null`, "must be a JSON object"},
		"string root":      {`"debug"`, "must be a JSON object"},
		"dotted key":       {`{"auth.enabled": true}`, `"auth.enabled": use nested objects`},
		"nested dotted":    {`{"access": {"tables.x": 1}}`, `"access.tables.x": use nested objects`},
		"dotted infra key": {`{"pg.host": "evil.example"}`, `"pg.host": use nested objects`},
		"scalar for table": {`{"access": false}`, `"access" must be an object`},
		"scalar for nested table": {
			`{"json": {"agg": "json_agg"}}`, `"json.agg" must be an object`,
		},
		"table for array":  {`{"access": {"tables": {"name": "orders"}}}`, `"access.tables" must not be an object`},
		"table for scalar": {`{"debug": {"enabled": true}}`, `"debug" must not be an object`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := loadWithConfigJSON(t, configJSONTestTOML, tc.doc)
			require.ErrorContains(t, err, tc.want)
			require.Nil(t, cfg)
		})
	}
}

func TestApplyConfigJSON_Size(t *testing.T) {
	pad := func(n int) string {
		prefix, suffix := `{"version": 1, "x": "`, `"}`
		return prefix + strings.Repeat("a", n-len(prefix)-len(suffix)) + suffix
	}

	_, err := loadWithConfigJSON(t, "", pad(configJSONMaxSize+1))
	require.ErrorContains(t, err, "the limit is 1048576")

	cfg, err := loadWithConfigJSON(t, "", pad(configJSONMaxSize))
	require.NoError(t, err, "a document at the limit loads (with a warning)")
	require.NotNil(t, cfg)
}

func TestApplyConfigJSON_Base64(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prest.toml")
	require.NoError(t, os.WriteFile(path, []byte(configJSONTestTOML), 0600))
	t.Setenv("PREST_CONF", path)
	unsetEnvForTest(t, EnvConfigJSON)

	t.Run("decoded", func(t *testing.T) {
		t.Setenv(EnvConfigJSONB64, base64.StdEncoding.EncodeToString([]byte(`{"context": "/b64"}`)))
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "/b64", cfg.ContextPath)
	})

	t.Run("line-wrapped", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte(`{"context": "/wrapped", "debug": true}`))
		t.Setenv(EnvConfigJSONB64, enc[:8]+"\n"+enc[8:]+"\n")
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "/wrapped", cfg.ContextPath)
	})

	t.Run("invalid base64", func(t *testing.T) {
		t.Setenv(EnvConfigJSONB64, "not base64!")
		_, err := Load()
		require.ErrorContains(t, err, "decode PREST_CONFIG_JSON_B64")
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Setenv(EnvConfigJSONB64, base64.StdEncoding.EncodeToString([]byte(`{`)))
		_, err := Load()
		require.ErrorContains(t, err, "parse PREST_CONFIG_JSON_B64")
	})

	t.Run("plain wins", func(t *testing.T) {
		t.Setenv(EnvConfigJSON, `{"context": "/plain"}`)
		t.Setenv(EnvConfigJSONB64, "not base64!")
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, "/plain", cfg.ContextPath)
	})
}

func TestApplyConfigJSON_EmptyIsUnset(t *testing.T) {
	cfg, err := loadWithConfigJSON(t, configJSONTestTOML, "  \n")
	require.NoError(t, err)
	require.Equal(t, "/api", cfg.ContextPath)
}

func TestApplyConfigJSON_FilelessBoot(t *testing.T) {
	t.Setenv("PREST_PG_HOST", "pg.example")
	t.Setenv("PREST_PG_DATABASE", "app")
	cfg, err := loadWithConfigJSON(t, "", `{
		"auth": {"enabled": true, "table": "app_users", "metadata": ["first_name"]},
		"jwt": {"key": "0123456789abcdef0123456789abcdef", "whitelist": ["^/auth$", "^/health$"]},
		"access": {
			"restrict": true,
			"tables": [{"name": "customers", "schema": "public", "permissions": ["read", "write"]}],
			"users": [{"name": "alice", "tables": [{"name": "customers", "permissions": ["read"]}]}]
		},
		"cache": {"enabled": false, "time": 30},
		"json": {"agg": {"type": "json_agg"}},
		"pluginmiddlewarelist": [{"file": "hello", "func": "Hello"}]
	}`)
	require.NoError(t, err)
	require.Equal(t, "pg.example", cfg.PGHost)
	require.Equal(t, "app", cfg.PGDatabase)
	require.Equal(t, 3000, cfg.HTTPPort)
	require.True(t, cfg.AuthEnabled)
	require.Equal(t, "app_users", cfg.AuthTable)
	require.Equal(t, []string{"first_name"}, cfg.AuthMetadata)
	require.Equal(t, "0123456789abcdef0123456789abcdef", cfg.JWTKey)
	require.Equal(t, []string{"^/auth$", "^/health$"}, cfg.JWTWhiteList)
	require.True(t, cfg.AccessConf.Restrict)
	require.Equal(t, []TablesConf{{Schema: "public", Name: "customers", Permissions: []string{"read", "write"}}},
		cfg.AccessConf.Tables)
	require.Equal(t, []UsersConf{{Name: "alice", Tables: []TablesConf{{Name: "customers", Permissions: []string{"read"}}}}},
		cfg.AccessConf.Users)
	require.Equal(t, 30, cfg.Cache.Time)
	require.Equal(t, jsonAgg, cfg.JSONAggType)
	require.Equal(t, []PluginMiddleware{{File: "hello", Func: "Hello"}}, cfg.PluginMiddlewareList)
}

// A schema set in the document counts as operator-set, exactly like one in
// prest.toml, so the MySQL adapter does not replace it with the database name.
func TestApplyConfigJSON_SchemaCountsAsSet(t *testing.T) {
	t.Setenv("PREST_ENGINE", "mysql")
	t.Setenv("PREST_DB_URL", "mysql://user:pass@127.0.0.1:3306/shop")
	cfg, err := loadWithConfigJSON(t, "", `{"auth": {"schema": "public"}}`)
	require.NoError(t, err)
	require.Equal(t, "public", cfg.AuthSchema)
	require.Equal(t, "shop", cfg.QueriesConf.Schema)
}

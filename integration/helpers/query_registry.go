package helpers

import (
	"net/http"
	"sort"
	"testing"

	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// QueryRegistryTarget is one engine's queries server (queries.storage = "database",
// testdata/prest_queries.toml).
type QueryRegistryTarget struct {
	BaseURL   string // QueriesServerURL(t)
	Database  string // alias for /_QUERIES/{database}/... and /_QUERIES/registry/{database}/...
	AdminUser string // listed in queries.register_admins
	AdminPass string
	JWTKey    string // signs the non-admin token
}

// Engine-neutral templates on test7: sqlVal binds $n on Postgres/TimescaleDB and ? on MySQL,
// so the same text proves each adapter's binding.
const (
	lifecycleRead    = `SELECT * FROM test7 WHERE name = {{sqlVal "field1"}}`
	lifecycleSurname = `SELECT surname FROM test7 WHERE name = {{sqlVal "field1"}}`
	lifecycleWrite   = `INSERT INTO test7 (name, surname) VALUES ({{sqlVal "field1"}}, {{sqlVal "field2"}})`
	lifecycleDelete  = `DELETE FROM test7 WHERE name = {{sqlVal "field1"}}`
	lifecycleRow     = "registry-lifecycle"
)

// RunQueryRegistryLifecycle exercises startup import, registry CRUD, execution
// through every verb, alias routing and error paths against a live queries server.
// Grants for itest/lifecycle live in testdata/prest_queries.toml.
func RunQueryRegistryLifecycle(t *testing.T, tg QueryRegistryTarget) {
	t.Helper()

	reg := tg.BaseURL + "/_QUERIES/registry"
	exec := tg.BaseURL + "/_QUERIES/itest/lifecycle"
	row := "?field1=" + lifecycleRow
	token := LoginToken(t, tg.BaseURL, tg.AdminUser, tg.AdminPass)
	lifecycle := map[string]string{
		"location":   "itest",
		"name":       "lifecycle",
		"read_sql":   lifecycleRead,
		"write_sql":  lifecycleWrite,
		"delete_sql": lifecycleDelete,
	}

	// Best effort: a failed run must not leave templates or rows behind for the
	// other tests sharing this server. Statuses are ignored on purpose.
	t.Cleanup(func() {
		for _, req := range []struct{ method, url string }{
			{http.MethodDelete, exec + row},
			{http.MethodDelete, reg + "/itest/lifecycle"},
			{http.MethodDelete, reg + "/" + tg.Database + "/itest/lifecycle_alias"},
		} {
			r, err := http.NewRequest(req.method, req.url, nil)
			if err != nil {
				continue
			}
			r.Header.Set("Authorization", "Bearer "+token)
			if resp, err := http.DefaultClient.Do(r); err == nil {
				resp.Body.Close()
			}
		}
	})

	t.Run("guards", func(t *testing.T) {
		// The registry is admin-only: no token is 401 and a valid non-admin JWT is 403.
		DoAuthRequest(t, reg, nil, http.MethodGet, "", http.StatusUnauthorized, "RegistryNoToken")
		nonAdmin, err := controllers.Token(auth.User{Username: "other@postgres.rest"}, tg.JWTKey)
		require.NoError(t, err)
		DoAuthRequest(t, reg, map[string]string{"location": "itest", "name": "denied", "read_sql": "SELECT 1"},
			http.MethodPost, nonAdmin, http.StatusForbidden, "RegistryNonAdmin")
	})

	t.Run("startup_import", func(t *testing.T) {
		// import_on_startup copied testdata/queries into this engine's prest_queries.
		// Imported rows have no alias; a lookup by alias falls back to them.
		DoAuthRequest(t, reg+"?location=fulltable", nil, http.MethodGet, token, http.StatusOK, "RegistryImported", "get_all")
		DoAuthRequest(t, reg+"/"+tg.Database+"/fulltable/get_all", nil, http.MethodGet, token, http.StatusOK,
			"RegistryImportedByAlias", "read_sql")
	})

	t.Run("execute_imported", func(t *testing.T) {
		// The imported get_all template runs from the table, with and without the
		// database segment that selects the adapter.
		DoAuthRequest(t, tg.BaseURL+"/_QUERIES/fulltable/get_all?field1=gopher", nil, http.MethodGet, token,
			http.StatusOK, "ExecuteImported", "gopher")
		DoAuthRequest(t, tg.BaseURL+"/_QUERIES/"+tg.Database+"/fulltable/get_all?field1=gopher", nil, http.MethodGet, token,
			http.StatusOK, "ExecuteImportedWithDB", "gopher")
	})

	t.Run("create", func(t *testing.T) {
		// Register a template with read, write and delete SQL at runtime.
		DoAuthRequest(t, reg, lifecycle, http.MethodPost, token, http.StatusCreated, "RegistryCreate", `"name":"lifecycle"`)
	})

	t.Run("write_then_read", func(t *testing.T) {
		// POST runs write_sql with bound values; GET runs read_sql and sees the row.
		DoAuthRequest(t, exec+row+"&field2=bound", nil, http.MethodPost, token, http.StatusOK, "ExecuteWrite")
		DoAuthRequest(t, exec+row, nil, http.MethodGet, token, http.StatusOK, "ExecuteRead", lifecycleRow, "bound")
	})

	t.Run("update_is_live", func(t *testing.T) {
		// PUT replaces every verb column, so it resends write/delete. The next GET
		// must run the new read_sql: no restart, no stale template.
		updated := map[string]string{"read_sql": lifecycleSurname, "write_sql": lifecycleWrite, "delete_sql": lifecycleDelete}
		DoAuthRequest(t, reg+"/itest/lifecycle", updated, http.MethodPut, token, http.StatusOK, "RegistryUpdate", "SELECT surname")

		var rows []map[string]any
		DoAuthRequestJSON(t, exec+row, nil, http.MethodGet, token, http.StatusOK, "ExecuteUpdatedRead", &rows)
		require.Len(t, rows, 1)
		require.Equal(t, []string{"surname"}, sortedKeys(rows[0]))
		require.Equal(t, "bound", rows[0]["surname"])
	})

	t.Run("delete_via_registry", func(t *testing.T) {
		// DELETE runs delete_sql, which also cleans up the row this test wrote.
		DoAuthRequest(t, exec+row, nil, http.MethodDelete, token, http.StatusOK, "ExecuteDelete")
		var rows []map[string]any
		DoAuthRequestJSON(t, exec+row, nil, http.MethodGet, token, http.StatusOK, "ExecuteReadAfterDelete", &rows)
		require.Empty(t, rows)
	})

	t.Run("alias_path", func(t *testing.T) {
		// Entries created on /registry/{database} are stored under that alias and
		// listed, read and deleted through the same path.
		aliasReg := reg + "/" + tg.Database
		DoAuthRequest(t, aliasReg, map[string]string{"location": "itest", "name": "lifecycle_alias", "read_sql": "SELECT 1"},
			http.MethodPost, token, http.StatusCreated, "RegistryCreateAlias")
		DoAuthRequest(t, aliasReg, nil, http.MethodGet, token, http.StatusOK, "RegistryListAlias", "lifecycle_alias")
		DoAuthRequest(t, aliasReg+"/itest/lifecycle_alias", nil, http.MethodGet, token, http.StatusOK, "RegistryGetAlias", "lifecycle_alias")
		DoAuthRequest(t, aliasReg+"/itest/lifecycle_alias", nil, http.MethodDelete, token, http.StatusNoContent, "RegistryDeleteAlias")
	})

	t.Run("validation", func(t *testing.T) {
		// A template needs at least one verb SQL, and the body must be JSON.
		DoAuthRequest(t, reg, map[string]string{"location": "itest", "name": "empty"}, http.MethodPost, token,
			http.StatusBadRequest, "RegistryCreateNoSQL")
		testutils.DoRequestRaw(t, reg, []byte(`{invalid`), http.MethodPost, http.StatusBadRequest, "RegistryInvalidJSON",
			map[string]string{"Authorization": "Bearer " + token})
	})

	t.Run("remove", func(t *testing.T) {
		// Deleting twice is 404, and the removed template can no longer run.
		DoAuthRequest(t, reg+"/itest/lifecycle", nil, http.MethodDelete, token, http.StatusNoContent, "RegistryDelete")
		DoAuthRequest(t, reg+"/itest/lifecycle", nil, http.MethodDelete, token, http.StatusNotFound, "RegistryDeleteAgain")
		DoAuthRequest(t, exec, nil, http.MethodGet, token, http.StatusBadRequest, "ExecuteAfterDelete")
	})
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

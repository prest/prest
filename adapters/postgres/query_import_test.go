package postgres

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestScriptPermissions_OpenMode(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{Restrict: false}}).(*postgres)
	require.True(t, adapter.ScriptPermissions("", "fulltable", "get_all", "read", ""))
}

func TestScriptPermissions_GlobalRule(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{
		Restrict: true,
		Scripts: []config.ScriptConf{{
			Location:    "fulltable",
			Name:        "get_all",
			Permissions: []string{"read"},
		}},
	}}).(*postgres)

	require.True(t, adapter.ScriptPermissions("", "fulltable", "get_all", "read", ""))
	require.False(t, adapter.ScriptPermissions("", "fulltable", "get_all", "write", ""))
	require.False(t, adapter.ScriptPermissions("", "other", "get_all", "read", ""))
}

func TestScriptPermissions_UserOverride(t *testing.T) {
	t.Parallel()

	adapter := New(&config.Prest{QueriesConf: config.QueriesConf{
		Restrict: true,
		Scripts: []config.ScriptConf{{
			Location:    "fulltable",
			Name:        "get_all",
			Permissions: []string{"read"},
		}},
		Users: []config.QueryUsersConf{{
			Name: "alice",
			Scripts: []config.ScriptConf{{
				Location:    "fulltable",
				Name:        "get_all",
				Permissions: []string{"read", "write"},
			}},
		}},
	}}).(*postgres)

	require.True(t, adapter.ScriptPermissions("", "fulltable", "get_all", "write", "alice"))
	require.False(t, adapter.ScriptPermissions("", "fulltable", "get_all", "write", "bob"))
}

func TestDiffStoredQuery(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{ReadSQL: "SELECT 1", WriteSQL: "INSERT 1"}
	incoming := adapters.StoredQuery{ReadSQL: "SELECT 1", WriteSQL: "INSERT 2"}

	changed, conflict, err := diffStoredQuery(existing, incoming)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, conflict)

	cols := diffColumns(existing, incoming)
	require.Equal(t, []string{"write_sql"}, cols)
}

func TestMergeStoredQuery_PreservesMissingFileColumns(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{ReadSQL: "SELECT 1", DeleteSQL: "DELETE 1"}
	incoming := adapters.StoredQuery{WriteSQL: "INSERT 1"}

	merged := mergeStoredQuery(existing, incoming)
	require.Equal(t, "SELECT 1", merged.ReadSQL)
	require.Equal(t, "INSERT 1", merged.WriteSQL)
	require.Equal(t, "DELETE 1", merged.DeleteSQL)
}

func TestScanFilesystemQueries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	loc := "fulltable"
	locDir := filepath.Join(dir, loc)
	require.NoError(t, os.MkdirAll(locDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "get_all.read.sql"), []byte("SELECT 1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "get_all.write.sql"), []byte("INSERT 1"), 0o600))

	queries, err := scanFilesystemQueries(dir)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "fulltable", queries[0].Location)
	require.Equal(t, "get_all", queries[0].Name)
	require.Equal(t, "SELECT 1", queries[0].ReadSQL)
	require.Equal(t, "INSERT 1", queries[0].WriteSQL)
	require.Equal(t, "filesystem-import", queries[0].CreatedBy)
}

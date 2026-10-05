package access

import (
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestTableAllowed(t *testing.T) {
	t.Parallel()
	cfg := &config.Prest{
		AccessConf: config.AccessConf{
			Restrict:    true,
			IgnoreTable: []string{"open"},
			Tables: []config.TablesConf{
				{Name: "items", Permissions: []string{"read"}},
				{Schema: "app", Name: "items", Permissions: []string{"read", "write"}},
				{Database: "db", Schema: "app", Name: "items", Permissions: []string{"delete"}},
			},
			Users: []config.UsersConf{{
				Name: "ada",
				Tables: []config.TablesConf{
					{Name: "items", Permissions: []string{"write"}},
				},
			}},
		},
	}

	require.True(t, TableAllowed(nil, "db", "app", "items", "read", ""))
	require.True(t, TableAllowed(cfg, "db", "app", "open", "write", ""))
	require.True(t, TableAllowed(cfg, "db", "other", "items", "read", ""))
	require.False(t, TableAllowed(cfg, "db", "app", "items", "write", ""))
	require.True(t, TableAllowed(cfg, "other", "app", "items", "write", ""))
	require.True(t, TableAllowed(cfg, "db", "app", "items", "delete", ""))
	require.False(t, TableAllowed(cfg, "db", "app", "missing", "read", ""))
	require.True(t, TableAllowed(cfg, "db", "app", "items", "write", "ada"))
	require.False(t, TableAllowed(cfg, "db", "app", "items", "delete", "ada"))
	require.False(t, TableAllowed(cfg, "db", "app", "items", "read", "ada"))
}

func TestFieldsFor(t *testing.T) {
	t.Parallel()
	cfg := &config.Prest{
		AccessConf: config.AccessConf{
			Tables: []config.TablesConf{
				{Name: "items", Permissions: []string{"read"}, Fields: []string{"id"}},
			},
			Users: []config.UsersConf{{
				Name: "ada",
				Tables: []config.TablesConf{
					{Name: "items", Permissions: []string{"read"}, Fields: []string{"name"}},
				},
			}},
		},
	}
	require.Equal(t, []string{"*"}, FieldsFor(nil, "", "", "t", "read", ""))
	require.Equal(t, []string{"*"}, FieldsFor(cfg, "", "", "other", "read", ""))
	require.Equal(t, []string{"id"}, FieldsFor(cfg, "", "", "items", "read", ""))
	require.Equal(t, []string{"name"}, FieldsFor(cfg, "", "", "items", "read", "ada"))
}

func TestScriptAllowed(t *testing.T) {
	t.Parallel()
	cfg := &config.Prest{
		QueriesConf: config.QueriesConf{
			Restrict: true,
			Scripts: []config.ScriptConf{
				{Location: "reports", Name: "daily", Permissions: []string{"read"}},
				{Database: "db", Location: "reports", Name: "daily", Permissions: []string{"write"}},
			},
			Users: []config.QueryUsersConf{{
				Name: "ada",
				Scripts: []config.ScriptConf{
					{Location: "*", Name: "*", Permissions: []string{"read"}},
				},
			}},
		},
	}
	require.True(t, ScriptAllowed(nil, "db", "reports", "daily", "read", ""))
	require.True(t, ScriptAllowed(cfg, "", "reports", "daily", "read", ""))
	require.True(t, ScriptAllowed(cfg, "db", "reports", "daily", "write", ""))
	require.False(t, ScriptAllowed(cfg, "db", "reports", "other", "read", ""))
	require.True(t, ScriptAllowed(cfg, "db", "reports", "other", "read", "ada"))
	require.False(t, ScriptAllowed(cfg, "db", "reports", "other", "write", "ada"))
}

package access

import (
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestReservedTable(t *testing.T) {
	t.Parallel()
	custom := &config.Prest{
		AuthTable:   "app_users",
		QueriesConf: config.QueriesConf{Table: "stored_sql"},
	}
	defaults := &config.Prest{
		AuthTable:   "prest_users",
		QueriesConf: config.QueriesConf{Table: "prest_queries"},
	}

	cases := []struct {
		description string
		cfg         *config.Prest
		table       string
		reserved    bool
	}{
		{"default queries table is reserved", defaults, "prest_queries", true},
		{"default auth table is reserved", defaults, "prest_users", true},
		{"upper-case spelling of the queries table is reserved", defaults, "PREST_QUERIES", true},
		{"mixed-case spelling of the auth table is reserved", defaults, "Prest_Users", true},
		{"ordinary table is not reserved", defaults, "items", false},
		{"empty table name is not reserved", defaults, "", false},
		{"configured queries table name is reserved", custom, "stored_sql", true},
		{"configured auth table name is reserved", custom, "app_users", true},
		{"default names stay reserved when custom names are configured", custom, "prest_queries", true},
		{"nil config still reserves the default queries table", nil, "prest_queries", true},
		{"nil config still reserves the default auth table", nil, "prest_users", true},
		{"empty config fields fall back to the defaults", &config.Prest{}, "prest_users", true},
		{"nil config leaves ordinary tables alone", nil, "items", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.reserved, ReservedTable(tc.cfg, tc.table), tc.description)
	}
}

// TestTableAllowed_ReservedTables guards GHSA-2x9q-r7j9-g85f: the stored-query
// and auth tables must be unreachable from data-plane routes whatever the ACL says.
func TestTableAllowed_ReservedTables(t *testing.T) {
	t.Parallel()
	open := &config.Prest{}
	granted := &config.Prest{
		AccessConf: config.AccessConf{
			Restrict:    true,
			IgnoreTable: []string{"prest_users"},
			Tables: []config.TablesConf{
				{Name: "prest_queries", Permissions: []string{"read", "write", "delete"}},
			},
			Users: []config.UsersConf{{
				Name:   "ada",
				Tables: []config.TablesConf{{Name: "prest_users", Permissions: []string{"read", "write", "delete"}}},
			}},
		},
	}

	for _, op := range []string{"read", "write", "delete"} {
		for _, table := range []string{"prest_queries", "prest_users", "PREST_QUERIES"} {
			require.False(t, TableAllowed(nil, "db", "public", table, op, ""), "nil config %s %s", op, table)
			require.False(t, TableAllowed(open, "db", "public", table, op, ""), "restrict=false %s %s", op, table)
			require.False(t, TableAllowed(open, "db", "other", table, op, ""), "other schema %s %s", op, table)
			require.False(t, TableAllowed(granted, "db", "public", table, op, ""), "table rule / ignore_table %s %s", op, table)
			require.False(t, TableAllowed(granted, "db", "public", table, op, "ada"), "user rule %s %s", op, table)
		}
	}
	require.True(t, TableAllowed(open, "db", "public", "items", "write", ""), "ordinary table stays open")
}

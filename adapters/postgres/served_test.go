package postgres

import (
	"testing"

	"github.com/lib/pq"
	"github.com/prest/prest/v2/adapters/postgres/statements"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

// TestDatabaseWhere_ServedDatabases checks that catalog listings are limited
// to the served databases (pg.database plus pg.allowed_databases) when no
// registry is configured, and that the request filter keeps its placeholders.
func TestDatabaseWhere_ServedDatabases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		cfg     *config.Prest
		request string
		suffix  string
	}{
		{
			name:   "default only",
			cfg:    &config.Prest{PGDatabase: "prest"},
			suffix: " AND datname IN ('prest')",
		},
		{
			name:   "allow-list",
			cfg:    &config.Prest{PGDatabase: "prest", AllowedDatabases: []string{"a", "b"}},
			suffix: " AND datname IN ('prest', 'a', 'b')",
		},
		{
			name:   "dedupe and empties",
			cfg:    &config.Prest{PGDatabase: "prest", AllowedDatabases: []string{"", "prest", "a"}},
			suffix: " AND datname IN ('prest', 'a')",
		},
		{
			name:    "with request",
			cfg:     &config.Prest{PGDatabase: "prest"},
			request: `"datname" = $1`,
			suffix:  ` AND datname IN ('prest') AND "datname" = $1`,
		},
		{
			name:   "quote",
			cfg:    &config.Prest{PGDatabase: "o'brien"},
			suffix: " AND datname IN ('o''brien')",
		},
		{
			name:   "backslash",
			cfg:    &config.Prest{PGDatabase: `a\b`},
			suffix: " AND datname IN (" + pq.QuoteLiteral(`a\b`) + ")",
		},
		{
			name:   "nothing served",
			cfg:    &config.Prest{PGDatabase: ""},
			suffix: " AND FALSE",
		},
		{
			name:   "registry",
			cfg:    &config.Prest{PGDatabase: "prest", Databases: []config.DatabaseConf{{Alias: "x"}}},
			suffix: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			adapter := testAdapter(tc.cfg)
			require.Equal(t, statements.DatabasesWhere+tc.suffix, adapter.DatabaseWhere(tc.request))
		})
	}
}

// TestAliases_NoRegistryIncludesAllowList checks that MCP list_databases sees
// pg.database plus the allow-list without a registry, and only the registry
// aliases when one is configured.
func TestAliases_NoRegistryIncludesAllowList(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{PGDatabase: "prest", AllowedDatabases: []string{"a", "", "prest", "b"}})
	require.Equal(t, []string{"prest", "a", "b"}, adapter.Aliases())

	adapter = testAdapter(&config.Prest{AllowedDatabases: []string{"a"}})
	require.Equal(t, []string{"a"}, adapter.Aliases())

	adapter = testAdapter(&config.Prest{AllowedDatabases: []string{""}})
	require.Nil(t, adapter.Aliases())

	adapter = testAdapter(&config.Prest{
		PGDatabase:       "prest",
		AllowedDatabases: []string{"a"},
		Databases:        []config.DatabaseConf{{Alias: "x"}, {Alias: "y"}},
	})
	require.Equal(t, []string{"x", "y"}, adapter.Aliases())
}

// TestDedupeNonEmpty checks the served-databases helper keeps the first
// occurrence, preserves order and drops empty names.
func TestDedupeNonEmpty(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a", "b"}, dedupeNonEmpty([]string{"", "a", "b", "a", ""}))
	require.NotNil(t, dedupeNonEmpty(nil))
	require.Empty(t, dedupeNonEmpty([]string{""}))
}

// TestServedFilter checks the inlined IN list and the FALSE fallback.
func TestServedFilter(t *testing.T) {
	t.Parallel()

	require.Equal(t, "FALSE", servedFilter("datname", nil))
	require.Equal(t, "datname IN ('x', 'y')", servedFilter("datname", []string{"x", "y"}))
}

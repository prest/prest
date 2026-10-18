package mysql

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

// TestDatabaseWhere_ServedDatabases restricts /databases to the served names:
// pg.database plus pg.allowed_databases without a registry, no filter with one.
func TestDatabaseWhere_ServedDatabases(t *testing.T) {
	t.Parallel()
	const base = " WHERE 1=1"
	cases := []struct {
		name    string
		cfg     *config.Prest
		request string
		want    string
	}{
		{"default only", &config.Prest{PGDatabase: "prest"}, "", base + " AND datname IN ('prest')"},
		{"allow-list", &config.Prest{PGDatabase: "prest", AllowedDatabases: []string{"a", "b"}}, "", base + " AND datname IN ('prest', 'a', 'b')"},
		{"dedupe and empty", &config.Prest{PGDatabase: "prest", AllowedDatabases: []string{"", "prest", "a"}}, "", base + " AND datname IN ('prest', 'a')"},
		{"with request", &config.Prest{PGDatabase: "prest"}, "`datname` = ?", base + " AND datname IN ('prest') AND `datname` = ?"},
		{"quote", &config.Prest{PGDatabase: "o'brien"}, "", base + " AND datname IN ('o''brien')"},
		{"backslash", &config.Prest{PGDatabase: `a\b`}, "", base + ` AND datname IN ('a\\b')`},
		{"nothing served", &config.Prest{}, "", base + " AND 1=0"},
		{"nil cfg", nil, "", base + " AND 1=0"},
		{"registry", &config.Prest{PGDatabase: "prest", Databases: []config.DatabaseConf{{Alias: "x"}}}, "", base},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Adapter{cfg: tc.cfg}
			require.Equal(t, tc.want, a.DatabaseWhere(tc.request))
		})
	}
}

// TestSchemaClause_ServedDatabases: on MySQL schemas are databases, so
// /schemas lists only the served ones; registry mode lists only the registered databases.
func TestSchemaClause_ServedDatabases(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	q, count := New(cfg).(*Adapter).SchemaClause(req("/schemas"))
	require.False(t, count)
	require.Contains(t, q, "AND SCHEMA_NAME IN ('shop', 'other')\n) s")
	q, count = New(cfg).(*Adapter).SchemaClause(req("/schemas?_count=schema_name"))
	require.True(t, count)
	require.Contains(t, q, "COUNT(schema_name)")
	require.Contains(t, q, "SCHEMA_NAME IN ('shop', 'other')")

	q, _ = (&Adapter{}).SchemaClause(req("/schemas"))
	require.Contains(t, q, "AND 1=0")

	reg := testCfg()
	reg.Databases = []config.DatabaseConf{{Alias: "x", Host: "db.internal", Port: 3306, Database: "shop"}}
	q, _ = New(reg).(*Adapter).SchemaClause(req("/schemas"))
	require.Contains(t, q, "AND SCHEMA_NAME IN ('shop')")
	require.NotContains(t, q, "1=0")
}

// TestAliases_NoRegistryIncludesAllowList: MCP list_databases sees every
// served name without a registry, and only the entry alias with one.
func TestAliases_NoRegistryIncludesAllowList(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other", "shop", ""}
	a := New(cfg).(*Adapter)
	a.SetDatabase("shop")
	require.Equal(t, []string{"shop", "other"}, a.Aliases())
	require.Nil(t, New(&config.Prest{}).(*Adapter).Aliases())
	require.Nil(t, (&Adapter{conn: New(nil).(*Adapter).conn}).Aliases())

	reg := testCfg()
	reg.Databases = []config.DatabaseConf{{Alias: "archive"}}
	r := New(reg).(*Adapter)
	require.Equal(t, []string{"shop"}, r.Aliases())
	r.SetDatabase("archive")
	require.Equal(t, []string{"archive"}, r.Aliases())
}

// TestPhysicalName_NoRegistryServedName: without a registry each served name
// is itself a MySQL database; a registry entry maps to its pg.database.
func TestPhysicalName_NoRegistryServedName(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	a := New(cfg).(*Adapter)
	require.Equal(t, "other", a.PhysicalName("other"))
	require.Equal(t, "shop", a.PhysicalName("shop"))
	require.Equal(t, "shop", a.PhysicalName(""))
	require.Equal(t, "shop", a.PhysicalName("unknown"))

	reg := testCfg()
	reg.AllowedDatabases = []string{"other"}
	reg.Databases = []config.DatabaseConf{{Alias: "other"}}
	require.Equal(t, "shop", New(reg).(*Adapter).PhysicalName("other"))
}

// TestAllowListedDatabaseOpensLazily: an allow-listed name gets its own pool,
// dialled on first request against the same [pg] host with the database swapped.
func TestAllowListedDatabaseOpensLazily(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	var dsns []string
	restore := SetDBConnectForTest(func(_, dsn string) (*sqlx.DB, error) {
		dsns = append(dsns, dsn)
		return sqlx.NewDb(raw, "sqlmock"), nil
	})
	t.Cleanup(restore)

	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	a := New(cfg).(*Adapter)
	require.True(t, a.IsRegistered("other"))
	require.False(t, a.IsRegistered("mysql"))
	_, err = a.dbFromCtx(context.WithValue(context.Background(), pctx.DBNameKey, "other"))
	require.NoError(t, err)
	require.Len(t, dsns, 1)
	require.Contains(t, dsns[0], "/other?")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestUnreachableDatabaseIsUnavailable: a dial failure on lazy open or on
// Connect is classified as unavailable and does not leak host or port.
func TestUnreachableDatabaseIsUnavailable(t *testing.T) {
	dialErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	restore := SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) { return nil, dialErr })
	t.Cleanup(restore)

	reg := testCfg()
	reg.Databases = []config.DatabaseConf{{Alias: "down"}}
	a := New(reg).(*Adapter)
	a.SetDatabase("down")

	err := a.Connect()
	require.Error(t, err)
	require.True(t, adapters.IsUnavailable(err))

	_, err = a.dbFromCtx(context.WithValue(context.Background(), pctx.DBNameKey, "down"))
	require.ErrorIs(t, err, adapters.ErrUnavailable)
	require.NotContains(t, err.Error(), "db.internal")
	require.NotContains(t, err.Error(), "3306")
}

// TestTableWhere_ServedDatabases limits /tables and MCP list_tables to the
// served databases' tables: one MySQL user often sees databases pREST must
// not name.
func TestTableWhere_ServedDatabases(t *testing.T) {
	t.Parallel()

	plain := testCfg()
	plain.AllowedDatabases = []string{"other"}

	reg := testCfg()
	reg.PGDatabase = "other"
	reg.Databases = []config.DatabaseConf{{Alias: "archive", Database: "other"}}

	cases := []struct {
		name    string
		adapter *Adapter
		request string
		want    string
	}{
		{"no registry", New(plain).(*Adapter), "", "WHERE 1=1 AND `schema` IN ('shop', 'other')"},
		{"registry entry", New(reg).(*Adapter), "", "WHERE 1=1 AND `schema` = 'other'"},
		{"nil cfg", &Adapter{}, "", "WHERE 1=1 AND 1=0"},
		{"with request", New(plain).(*Adapter), "`name` = ?", "WHERE 1=1 AND `schema` IN ('shop', 'other') AND `name` = ?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.adapter.TableWhere(tc.request))
		})
	}
}

package postgres

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// dbAliasAttributes tags spans with the database alias from the request context.
func TestDBAliasAttributes(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "tenant-a")
	attrs := dbAliasAttributes(ctx, "", "", nil)
	require.Len(t, attrs, 1)
	require.Equal(t, "tenant-a", attrs[0].Value.AsString())

	// No alias in context -> no attribute emitted.
	require.Nil(t, dbAliasAttributes(context.Background(), "", "", nil))
}

// RegisterOTelDriver wraps the postgres driver and points the pool at it.
// It mutates connection globals; restore them so other tests see the default.
func TestRegisterOTelDriver(t *testing.T) {
	t.Cleanup(func() {
		connection.SetDriverName("postgres")
		connection.SetAfterConnect(nil)
	})

	cfg := &config.Prest{}
	cfg.Otel.Enabled = true
	require.NoError(t, RegisterOTelDriver(cfg))

	// Verify the pool now opens connections with the instrumented (non-default)
	// driver name.
	var gotDriver string
	restore := connection.SetDBConnectForTest(func(name, _ string) (*sqlx.DB, error) {
		gotDriver = name
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	})
	t.Cleanup(restore)

	m := connection.NewManager(&config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"})
	db, err := m.AddDatabaseToPool("testdb")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NotEqual(t, "postgres", gotDriver)
	require.Contains(t, gotDriver, "postgres")
}

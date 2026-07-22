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

// When OTel is enabled, New builds a Manager that opens connections through the
// instrumented (non-default) driver. When disabled, it uses the default driver.
func TestNew_OTelInstrumentsDriver(t *testing.T) {
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

	connectWith := func(cfg *config.Prest) string {
		gotDriver = ""
		p := New(cfg).(*postgres)
		db, err := p.conn.AddDatabaseToPool("testdb")
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		p.conn.ResetPoolForTest()
		return gotDriver
	}

	// Disabled: default lib/pq "postgres" driver.
	require.Equal(t, "postgres", connectWith(&config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"}))

	// Enabled: an otelsql-wrapped driver whose name is derived from "postgres".
	enabled := &config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"}
	enabled.Otel.Enabled = true
	got := connectWith(enabled)
	require.NotEqual(t, "postgres", got)
	require.Contains(t, got, "postgres")
}

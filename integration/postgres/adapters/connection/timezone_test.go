package connection_test

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// With PGTimezone set, every connection the pool opens (3 held at once) starts with that session TimeZone.
func TestTimezone_EveryPooledConnection(t *testing.T) {
	base := helpers.LoadTestConfig(t)
	cfg := *base
	cfg.PGTimezone = "Asia/Tokyo"
	cfg.PGMaxOpenConn = 3

	pg := postgres.New(&cfg)
	db, err := postgres.DB(pg)
	require.NoError(t, err)
	t.Cleanup(func() { postgres.Close(pg) })

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		c, err := db.Conn(ctx)
		require.NoError(t, err)
		defer c.Close()
		var tz string
		require.NoError(t, c.QueryRowContext(ctx, "SHOW TIME ZONE").Scan(&tz))
		require.Equal(t, "Asia/Tokyo", tz, "conn %d", i)
	}
}

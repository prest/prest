package connection

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestBuildDSN(t *testing.T) {
	t.Parallel()

	dsn, err := BuildDSN("app", "s3cret", "db.internal", 3306, "shop", "disable", "", "", "")
	require.NoError(t, err)
	require.Contains(t, dsn, "app:")
	require.Contains(t, dsn, "@tcp(db.internal:3306)/shop")
	require.Contains(t, dsn, "parseTime=true")
	require.Contains(t, dsn, "charset=utf8mb4")
	require.Contains(t, dsn, "multiStatements=false")
	require.Contains(t, dsn, "interpolateParams=false")
	require.Contains(t, dsn, "tls=false")
	require.NotContains(t, RedactedDSN(dsn), "s3cret")

	requireDSN := func(mode, want string) {
		t.Helper()
		got, err := BuildDSN("u", "p", "h", 0, "d", mode, "", "", "")
		require.NoError(t, err)
		require.Contains(t, got, "tcp(h:3306)")
		require.Contains(t, got, "tls="+want)
	}
	requireDSN("require", "true")
	requireDSN("skip-verify", "skip-verify")
	requireDSN("", "false")

	_, err = BuildDSN("", "p", "h", 3306, "d", "disable", "", "", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "h", 3306, "", "disable", "", "", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "", 3306, "d", "disable", "", "", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "h", 3306, "d", "verify-full", "", "", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "h", 3306, "d", "disable", "/cert", "", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "h", 3306, "d", "disable", "", "/key", "")
	require.Error(t, err)
	_, err = BuildDSN("u", "p", "h", 3306, "d", "disable", "", "", "/ca")
	require.Error(t, err)
}

func TestBuildDSNPasswordRoundTrip(t *testing.T) {
	t.Parallel()
	const pass = "p@ss/word"
	dsn, err := BuildDSN("app", pass, "localhost", 3306, "shop", "disable", "", "", "")
	require.NoError(t, err)
	require.Contains(t, dsn, "@tcp(localhost:3306)/shop")
	cfg, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, pass, cfg.Passwd)
	require.Equal(t, "app", cfg.User)
	require.Equal(t, "shop", cfg.DBName)
	require.NotContains(t, RedactedDSN(dsn), pass)
}

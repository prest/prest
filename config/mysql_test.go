package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyMySQLURLToPrestInvalidPort(t *testing.T) {
	t.Parallel()
	c := &Prest{PGHost: "keep", PGURL: "mysql://app:s3cret@db.internal:999999999999999999999/shop"}
	err := applyMySQLURLToPrest(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot parse mysql url port")
	require.Equal(t, "keep", c.PGHost)
	require.Empty(t, c.PGUser)
	require.Empty(t, c.PGPass)
	require.Empty(t, c.PGDatabase)
}

func TestApplyMySQLURLToPrestNonNumericPort(t *testing.T) {
	t.Parallel()
	raw := "mysql://app:s3cret@db.internal:abc/shop"
	c := &Prest{PGHost: "keep", PGURL: raw}
	err := applyMySQLURLToPrest(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot parse mysql url")
	require.NotContains(t, err.Error(), "s3cret")
	require.NotContains(t, err.Error(), raw)
	require.Equal(t, "keep", c.PGHost)
	require.Empty(t, c.PGUser)
	require.Empty(t, c.PGPass)
	require.Empty(t, c.PGDatabase)
}

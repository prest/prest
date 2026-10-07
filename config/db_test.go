package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseDatabaseURL(t *testing.T) {
	t.Parallel()

	t.Run("valid URL with sslmode", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user:pass@localhost:5432/mydatabase/?sslmode=require"}
		parseDatabaseURL(c)
		require.Equal(t, "mydatabase", c.PGDatabase)
		require.Equal(t, 5432, c.PGPort)
		require.Equal(t, "user", c.PGUser)
		require.Equal(t, "pass", c.PGPass)
		require.Equal(t, "require", c.PGSSLMode)
	})

	t.Run("empty URL is a no-op", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGHost: "keep", PGDatabase: "keep"}
		parseDatabaseURL(c)
		require.Equal(t, "keep", c.PGHost)
		require.Equal(t, "keep", c.PGDatabase)
	})

	t.Run("invalid port aborts URL parsing", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user:pass@localhost:999999999999999999999/mydatabase/?sslmode=require"}
		parseDatabaseURL(c)
		require.Equal(t, "localhost", c.PGHost)
		require.Empty(t, c.PGDatabase)
	})

	t.Run("invalid URL", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: `invalid%+o`}
		parseDatabaseURL(c)
		require.Equal(t, "", c.PGDatabase)
		require.Equal(t, "", c.PGUser)
	})

	t.Run("URL without password", func(t *testing.T) {
		t.Parallel()
		c := &Prest{PGURL: "postgresql://user@localhost/mydatabase"}
		parseDatabaseURL(c)
		require.Equal(t, "mydatabase", c.PGDatabase)
		require.Equal(t, "user", c.PGUser)
		require.Empty(t, c.PGPass)
	})
}

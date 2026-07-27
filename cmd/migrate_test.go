package cmd

import (
	"testing"

	"github.com/prest/prest/v2/config"

	"github.com/stretchr/testify/require"
)

// resolveURLConn mutates the package-level urlConn var, so this test cannot
// run in parallel with others that touch it.
func TestResolveURLConn(t *testing.T) {
	orig := urlConn
	t.Cleanup(func() { urlConn = orig })

	cfg := &config.Prest{
		PGUser: "app", PGPass: "super-secret-pass", PGHost: "localhost",
		PGPort: 5432, PGDatabase: "prest", PGSSLMode: "disable",
	}

	t.Run("leaves an explicitly-set --url alone", func(t *testing.T) {
		urlConn = "postgres://explicit-override"
		resolveURLConn(cfg)
		require.Equal(t, "postgres://explicit-override", urlConn)
	})

	t.Run("derives from config when --url was not set", func(t *testing.T) {
		urlConn = ""
		resolveURLConn(cfg)
		require.Equal(t, driverURL(cfg), urlConn)
		require.Contains(t, urlConn, "super-secret-pass")
	})
}

// TestMigrateURLFlagDefault_NeverBakesInCredentials guards against the
// plaintext-password-in---help leak: the flag registered in Execute (see
// root.go) must use an empty default, with resolveURLConn filling it in later
// from cfg — never driverURL(cfg) baked directly into the flag default, which
// cobra echoes verbatim in `prestd migrate --help`.
func TestMigrateURLFlagDefault_NeverBakesInCredentials(t *testing.T) {
	orig := urlConn
	t.Cleanup(func() { urlConn = orig })
	urlConn = ""

	f := migrateCmd.PersistentFlags().Lookup("url")
	require.NotNil(t, f)
	require.Empty(t, f.DefValue)
}

package helpers

import (
	"os"
	"strings"
	"testing"
)

const deploySkipMsg = "deployed prestd not configured; run make test-integration"

// ServerURL returns the base URL for the default integration prestd service.
func ServerURL(t *testing.T) string {
	t.Helper()
	return envURL(t, "PREST_TEST_URL")
}

// MultiClusterServerURL returns the base URL for the multi-cluster prestd service.
func MultiClusterServerURL(t *testing.T) string {
	t.Helper()
	if SecondaryClusterHost() == "" {
		t.Skip("secondary postgres cluster not configured")
	}
	return envURL(t, "PREST_MULTICLUSTER_TEST_URL")
}

// AuthServerURL returns the base URL for the auth-enabled prestd service.
func AuthServerURL(t *testing.T) string {
	t.Helper()
	return envURL(t, "PREST_AUTH_TEST_URL")
}

func envURL(t *testing.T, key string) string {
	t.Helper()
	u := strings.TrimSpace(os.Getenv(key))
	if u == "" {
		t.Skip(deploySkipMsg)
	}
	return strings.TrimRight(u, "/")
}

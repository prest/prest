package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestMCPMultiClusterDiscovery(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)

	// Discover MCP tools on the multicluster prestd.
	// Expected to succeed with HTTP status OK and advertise tools.
	testutils.DoRequest(
		t, base+"/_mcp",
		nil, http.MethodGet, http.StatusOK, "MCPMultiClusterDiscovery",
		`"name":"prest"`,
		`"tools"`,
	)
}

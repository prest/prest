package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestCheckDBHealth(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_health", nil, "GET", http.StatusOK, "CheckDBHealth")
}

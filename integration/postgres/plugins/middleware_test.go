package plugins_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestPluginsMiddleware(t *testing.T) {
	base := helpers.ServerURL(t)

	// GET a CRUD route on the compose prestd service. Plugin middleware is
	// wired on the CRUD stack (middlewares.NewCRUDStack); expected: 200 and
	// X-Hello-Middleware from lib/src/middlewares/hello.so built at prestd start.
	testutils.DoRequestExpectResponseHeaders(
		t,
		base+"/prest-test/public/test",
		nil,
		http.MethodGet,
		http.StatusOK,
		"PluginsMiddleware",
		nil,
		map[string]string{"X-Hello-Middleware": "Hello Middleware"},
	)
}

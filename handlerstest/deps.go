package handlerstest

import (
	"testing"

	"github.com/prest/prest/v2/controllers"
)

// NewTestHandlers constructs handlers from injected dependencies for unit tests.
func NewTestHandlers(t *testing.T, deps controllers.Deps) *controllers.Handlers {
	t.Helper()
	return controllers.NewHandlers(deps, nil)
}

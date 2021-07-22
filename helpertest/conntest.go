package helpertest

import (
	"context"
	"testing"

	"zombiezen.com/go/postgrestest"
)

// Run ... tests
func Run(ctx context.Context, t *testing.T) (srv *postgrestest.Server) {
	srv, err := postgrestest.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return
}

// Close go get zombiezen.com/go/postgrestest
func Close(t *testing.T, f func()) {
	defer func() {
		t.Cleanup(f)
	}()
}

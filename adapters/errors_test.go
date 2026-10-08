package adapters

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("syntax error"), false},
		{"relation not found", ErrRelationNotFound, false},
		{"sentinel", ErrUnavailable, true},
		{"wrapped sentinel", fmt.Errorf("query: %w", ErrUnavailable), true},
		{"bad conn", driver.ErrBadConn, true},
		{"refused", fmt.Errorf("x: %w", syscall.ECONNREFUSED), true},
		{"dial op error", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, true},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, IsUnavailable(tc.err), tc.name)
	}
}

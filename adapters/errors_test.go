package adapters

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

var dnsErr = &net.DNSError{Err: "no such host", Name: "db.invalid", IsNotFound: true}

// timeoutErr is a net.Error that reports a timeout, as a dial deadline does.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

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
		// lib/pq dials with net.Dialer, so DNS failures and connect_timeout
		// both arrive as *net.OpError.
		{"dial dns error", fmt.Errorf("connect: %w", &net.OpError{Op: "dial", Net: "tcp", Err: dnsErr}), true},
		{"dial timeout", fmt.Errorf("connect: %w", &net.OpError{Op: "dial", Net: "tcp", Err: timeoutErr{}}), true},
		// A resolver error that is not wrapped in an OpError still means the
		// host cannot be reached.
		{"bare dns error", fmt.Errorf("lookup: %w", dnsErr), true},
		// A statement deadline is a slow query, not an outage; it keeps its
		// existing 400 mapping.
		{"context deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, IsUnavailable(tc.err), tc.name)
	}
}

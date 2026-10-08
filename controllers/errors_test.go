package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

func TestJsonError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	jsonError(rec, "something failed", http.StatusTeapot)

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, `{"error":"something failed"}`+"\n", rec.Body.String())
}

// A message carrying a double quote used to be interpolated straight into the
// body, producing JSON a client cannot parse. Escaping keeps the body valid
// whatever the message contains.
func TestJSONError_EscapesMessage(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	jsonError(rec, `invalid value for parameter "slug"`, http.StatusBadRequest)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body must be valid JSON")
	require.Equal(t, `invalid value for parameter "slug"`, body.Error)
}

// dialError is what database/sql surfaces when the server refuses a
// connection; its text carries host:port, which must not reach clients.
func dialError() error {
	return &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 5432}, Err: syscall.ECONNREFUSED}
}

func TestStatusFor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		err    error
		status int
		msg    string
	}{
		{"nil", nil, http.StatusOK, ""},
		{"sentinel unavailable", adapters.ErrUnavailable, http.StatusServiceUnavailable, "database unavailable"},
		{"wrapped unavailable", fmt.Errorf("query: %w", adapters.ErrUnavailable), http.StatusServiceUnavailable, "database unavailable"},
		{"net op error", dialError(), http.StatusServiceUnavailable, "database unavailable"},
		{"relation sentinel", fmt.Errorf("x: %w", adapters.ErrRelationNotFound), http.StatusNotFound, "x: relation not found"},
		{"pq relation", errors.New(`pq: relation "public.users" does not exist`), http.StatusNotFound, `pq: relation "public.users" does not exist`},
		{"plain", errors.New("syntax error"), http.StatusBadRequest, "syntax error"},
	} {
		status, msg := statusFor(tc.err, "public", "users")
		require.Equal(t, tc.status, status, tc.name)
		require.Equal(t, tc.msg, msg, tc.name)
	}
}

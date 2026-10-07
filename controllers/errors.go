package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/internal/logsafe"
)

var (
	ErrUserNotFound            = errors.New(unf)
	ErrUnknownEncryptAlgorithm = errors.New("unknown encrypt algorithm")
	ErrAuthDialectMissing      = errors.New("auth: no SQL dialect configured")
	jsonErrorMsg               = `{"error":"%s"}`
)

const (
	unf = "user not found"
	// unavailableMsg replaces driver text on connection failures, which
	// carries the database host:port.
	unavailableMsg = "database unavailable"
)

// jsonError writes message as a JSON error body.
//
// The message is escaped rather than interpolated raw: a message containing a
// double quote — or any control character — would otherwise emit a body that is
// not valid JSON, which clients cannot parse to discover what went wrong.
func jsonError(writer http.ResponseWriter, message string, status int) {
	encoded, err := json.Marshal(message)
	if err != nil {
		// Marshalling a string is not expected to fail; fall back to a fixed,
		// certainly-valid body rather than emitting an unchecked one.
		http.Error(writer, fmt.Sprintf(jsonErrorMsg, "request failed"), status)
		return
	}
	http.Error(writer, fmt.Sprintf(`{"error":%s}`, encoded), status)
}

// statusFor maps a statement error to its HTTP status and public message:
// connection failures are 503 with a fixed message, a missing relation is 404
// and anything else is 400 with the error text. A nil error is 200.
func statusFor(err error, schema, table string) (int, string) {
	switch {
	case err == nil:
		return http.StatusOK, ""
	case adapters.IsUnavailable(err):
		return http.StatusServiceUnavailable, unavailableMsg
	case isRelationNotFound(err, schema, table):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusBadRequest, err.Error()
	}
}

// writeStatementError answers a failed statement using statusFor. The
// prefixes keep each endpoint's existing 404/400 messages; 503 never carries
// one and logs the redacted cause instead.
func writeStatementError(w http.ResponseWriter, err error, schema, table, notFoundPrefix, badRequestPrefix string) {
	status, msg := statusFor(err, schema, table)
	switch status {
	case http.StatusServiceUnavailable:
		slog.Error(unavailableMsg, "err", logsafe.Error(err))
	case http.StatusNotFound:
		msg = notFoundPrefix + msg
	default:
		msg = badRequestPrefix + msg
	}
	jsonError(w, msg, status)
}

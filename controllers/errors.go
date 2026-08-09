package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUserNotFound            = errors.New(unf)
	ErrUnknownEncryptAlgorithm = errors.New("unknown encrypt algorithm")
	jsonErrorMsg               = `{"error":"%s"}`
)

const (
	unf = "user not found"
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

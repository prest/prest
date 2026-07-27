package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/prest/prest/v2/queryguard"
)

var (
	ErrUserNotFound            = errors.New(unf)
	ErrUnknownEncryptAlgorithm = errors.New("unknown encrypt algorithm")
	jsonErrorMsg               = `{"error":"%s"}`
)

const (
	unf = "user not found"
)

func jsonError(writer http.ResponseWriter, message string, status int) {
	http.Error(writer, fmt.Sprintf(jsonErrorMsg, message), status)
}

// guardRejection is the body returned when Query Guard refuses a query. The rule
// is included so clients can tell which limit they hit without parsing prose.
type guardRejection struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
	Rule   string `json:"rule"`
}

// newGuardRejection renders a rejection as the payload shared by the REST and
// MCP error paths, so both report the same rule and reason.
func newGuardRejection(rejection *queryguard.RejectionError) guardRejection {
	return guardRejection{
		Error:  queryguard.ErrRejected.Error(),
		Reason: rejection.Reason,
		Rule:   rejection.Rule,
	}
}

// jsonGuardError answers a refused query with 422 Unprocessable Entity: the
// request is well formed and authorized, but its execution plan is not
// acceptable. That keeps it distinguishable from a malformed request (400) and
// from a permission denial (403).
func jsonGuardError(writer http.ResponseWriter, rejection *queryguard.RejectionError) {
	body, err := json.Marshal(newGuardRejection(rejection))
	if err != nil {
		jsonError(writer, rejection.Error(), http.StatusUnprocessableEntity)
		return
	}
	http.Error(writer, string(body), http.StatusUnprocessableEntity)
}

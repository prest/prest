package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

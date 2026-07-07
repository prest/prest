package controllers

import (
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

package controllers

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// failingWriter is a ResponseWriter whose body writes fail, as when the client
// has gone away.
type failingWriter struct {
	*httptest.ResponseRecorder
}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write: broken pipe")
}

func TestWriteBody_WritesPayload(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeBody(w, []byte(`{"ok":true}`))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `{"ok":true}`, w.Body.String())
}

// Not parallel: swaps the default slog logger.
func TestWriteBody_LogsFailedWrite(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	writeBody(failingWriter{httptest.NewRecorder()}, []byte(`{"ok":true}`))

	require.Contains(t, buf.String(), "could not write response body")
	require.Contains(t, buf.String(), "broken pipe")
}

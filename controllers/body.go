package controllers

import (
	"log/slog"
	"net/http"

	"github.com/prest/prest/v2/internal/logsafe"
)

// writeBody writes body as the response payload. By then the status line has
// been sent, so a failed write (usually a client that went away) cannot be
// reported to the client; it is logged instead of being silently dropped.
func writeBody(w http.ResponseWriter, body []byte) {
	if _, err := w.Write(body); err != nil {
		slog.Warn("could not write response body", "err", logsafe.Error(err))
	}
}

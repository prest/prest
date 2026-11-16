package middlewares

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/urfave/negroni/v3"

	pctx "github.com/prest/prest/v2/context"
)

// maxRequestIDLength bounds an incoming request ID; a longer value is replaced
// by a generated one so a client cannot bloat every log line it reaches.
const maxRequestIDLength = 200

// RequestIDMiddleware gives every request a request (correlation) ID carried
// in header, e.g. X-Request-Id. The ID a client or proxy sent is kept when it
// is 1 to 200 visible ASCII characters; otherwise a random 32-character hex ID
// is generated. The ID is set on the request header seen by later handlers and
// the access log, stored in the request context under pctx.RequestIDKey, and
// sent back in the same response header, errors included.
func RequestIDMiddleware(header string) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		id := r.Header.Get(header)
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set(header, id)

		r = r.WithContext(context.WithValue(r.Context(), pctx.RequestIDKey, id))
		// WithContext shares the header map; clone it so the ID is not
		// written into the request the server handed in.
		r.Header = r.Header.Clone()
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set(header, id)
		next(w, r)
	})
}

// RequestIDFromContext returns the request ID RequestIDMiddleware stored in
// ctx, or "" when request IDs are disabled.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(pctx.RequestIDKey).(string)
	return id
}

// validRequestID reports whether a client-sent request ID can be kept: 1 to
// maxRequestIDLength printable ASCII characters other than space, which keeps
// control characters and line breaks out of logs.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '!' || id[i] > '~' {
			return false
		}
	}
	return true
}

// newRequestID returns 128 random bits as 32 lower-case hex characters.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// requestIDLogFormat is the access log format with the request ID of header
// appended as the last field.
func requestIDLogFormat(header string) string {
	return negroni.LoggerDefaultFormat + " | {{.Request.Header.Get " + strconv.Quote(header) + "}}"
}

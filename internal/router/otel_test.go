package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/stretchr/testify/require"
)

// The middleware renames the active span to the matched route template and
// records it as the http.route attribute (bounded label, no raw URL/PII).
func TestOtelRouteTagMiddleware_setsRouteOnSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	tracer := tp.Tracer("test")

	r := mux.NewRouter()
	r.Use(otelRouteTagMiddleware)
	r.HandleFunc("/{database}/{schema}/{table}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/db/public/users", nil)
	ctx, span := tracer.Start(req.Context(), "server")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	span.End()

	require.Equal(t, http.StatusOK, rec.Code)

	ended := sr.Ended()
	require.Len(t, ended, 1)
	const wantRoute = "/{database}/{schema}/{table}"
	require.Equal(t, "GET "+wantRoute, ended[0].Name())

	var gotRoute string
	for _, attr := range ended[0].Attributes() {
		if attr.Key == "http.route" {
			gotRoute = attr.Value.AsString()
		}
	}
	require.Equal(t, wantRoute, gotRoute)
}

// Unmatched routes leave the span untouched and do not panic.
func TestOtelRouteTagMiddleware_noRouteNoop(t *testing.T) {
	handler := otelRouteTagMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Called directly (not via mux), so no route is matched: must be a no-op.
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusTeapot, rec.Code)
}

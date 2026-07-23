package controllers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/stretchr/testify/require"
)

// installSpanRecorder makes a recording TracerProvider global for the test and
// restores the previous one on cleanup. Mutates global state: not parallel-safe.
func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

// A JSON-RPC call emits a span named mcp.rpc/<method> with the rpc.method attr.
func TestMCPHandler_EmitsRPCSpan(t *testing.T) {
	sr := installSpanRecorder(t)

	h := NewMCPHandler(Deps{})
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", bytes.NewBufferString(body)))
	require.Equal(t, http.StatusOK, rec.Code)

	ended := sr.Ended()
	require.Len(t, ended, 1)
	require.Equal(t, "mcp.rpc/initialize", ended[0].Name())

	var method string
	for _, a := range ended[0].Attributes() {
		if a.Key == "rpc.method" {
			method = a.Value.AsString()
		}
	}
	require.Equal(t, "initialize", method)
}

// tools/call produces a child tool span; a failing tool marks both Error.
func TestMCPHandler_EmitsToolSpanWithError(t *testing.T) {
	sr := installSpanRecorder(t)

	h := NewMCPHandler(Deps{})
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.nope"}}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", bytes.NewBufferString(body)))

	status := map[string]codes.Code{}
	for _, s := range sr.Ended() {
		status[s.Name()] = s.Status().Code
	}
	require.Contains(t, status, "mcp.rpc/tools/call")
	require.Contains(t, status, "mcp.tool/prest.nope")
	require.Equal(t, codes.Error, status["mcp.tool/prest.nope"])
}

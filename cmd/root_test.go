package cmd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prest/prest/v2/config"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/stretchr/testify/require"
)

// freeAddr returns a currently-free loopback address.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// waitUntilServing blocks until addr accepts a TCP connection or the deadline.
func waitUntilServing(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never came up", addr)
}

// Regression: a non-root context path must not clobber the route-template span
// name. contextPathHandler uses StripPrefix (not http.ServeMux, which would set
// http.Request.Pattern and make otelhttp overwrite the name with "GET /").
func TestContextPathHandler_PreservesRouteSpanName(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	// Minimal stand-in for the real router: name the span by the matched route
	// template, exactly as router.otelRouteTagMiddleware does.
	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if route := mux.CurrentRoute(req); route != nil {
				if tmpl, err := route.GetPathTemplate(); err == nil {
					trace.SpanFromContext(req.Context()).SetName(req.Method + " " + tmpl)
				}
			}
			next.ServeHTTP(w, req)
		})
	})
	r.HandleFunc("/{database}/{schema}/{table}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mirror app wiring: otelhttp wraps the router, then the context-path mount.
	handler := contextPathHandler("/api", otelhttp.NewHandler(r, "prest"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/mydb/public/users", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	ended := sr.Ended()
	require.Len(t, ended, 1)
	require.Equal(t, "GET /{database}/{schema}/{table}", ended[0].Name())
}

// Cancelling the context triggers a graceful shutdown and serveWithShutdown
// returns without error.
func TestServeWithShutdown_ContextCancellationShutsDown(t *testing.T) {
	addr := freeAddr(t)
	srv := &http.Server{Addr: addr, Handler: http.NotFoundHandler()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, &config.Prest{}, srv) }()

	waitUntilServing(t, addr)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serveWithShutdown did not return after context cancellation")
	}
}

// An unexpected serve failure (address already in use) is returned to the caller
// instead of being swallowed.
func TestServeWithShutdown_ServeErrorReturned(t *testing.T) {
	// Hold the port so ListenAndServe fails to bind.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	srv := &http.Server{Addr: ln.Addr().String(), Handler: http.NotFoundHandler()}
	err = serveWithShutdown(context.Background(), &config.Prest{}, srv)
	require.Error(t, err)
}

// A request still in flight past the grace period does not block shutdown: the
// grace period caps the wait and serveWithShutdown returns promptly.
func TestServeWithShutdown_ShutdownTimeout(t *testing.T) {
	prev := shutdownGracePeriod
	shutdownGracePeriod = 100 * time.Millisecond
	t.Cleanup(func() { shutdownGracePeriod = prev })

	addr := freeAddr(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var enterOnce sync.Once
	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			enterOnce.Do(func() { close(entered) }) // signal the request reached the handler
			<-release
		}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, &config.Prest{}, srv) }()
	waitUntilServing(t, addr)

	// Kick off a request that hangs in the handler.
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Get("http://" + addr + "/"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-entered // deterministically wait until the request is in-flight in the handler

	start := time.Now()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
		// Returned near the grace period, not blocked on the hung request.
		require.Less(t, time.Since(start), 2*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("serveWithShutdown blocked past the shutdown grace period")
	}
}

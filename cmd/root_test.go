package cmd

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prest/prest/v2/config"
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
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	srv := &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, &config.Prest{}, srv) }()
	waitUntilServing(t, addr)

	// Kick off a request that hangs in the handler.
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		if resp, err := client.Get("http://" + addr + "/"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	time.Sleep(100 * time.Millisecond) // let the request reach the handler

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

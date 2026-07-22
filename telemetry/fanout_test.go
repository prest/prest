package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/prest/prest/v2/config"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/stretchr/testify/require"
)

// A record is delivered to every wrapped handler.
func TestFanoutHandler_writesToAll(t *testing.T) {
	t.Parallel()

	var a, b bytes.Buffer
	logger := slog.New(newFanoutHandler(
		slog.NewJSONHandler(&a, nil),
		slog.NewJSONHandler(&b, nil),
	))

	logger.Info("hello", "k", "v")

	for _, out := range []string{a.String(), b.String()} {
		require.Contains(t, out, "hello")
		require.Contains(t, out, `"k":"v"`)
	}
}

// WithAttrs/WithGroup propagate to all wrapped handlers.
func TestFanoutHandler_withAttrsAndGroup(t *testing.T) {
	t.Parallel()

	var a, b bytes.Buffer
	logger := slog.New(newFanoutHandler(
		slog.NewJSONHandler(&a, nil),
		slog.NewJSONHandler(&b, nil),
	)).With("svc", "prest").WithGroup("g")

	logger.Info("m", "x", 1)

	for _, out := range []string{a.String(), b.String()} {
		require.Contains(t, out, `"svc":"prest"`)
		require.Contains(t, out, `"g":{"x":1}`)
	}
}

// Enabled is the OR of the wrapped handlers' levels.
func TestFanoutHandler_enabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := newFanoutHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	require.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	require.True(t, h.Enabled(context.Background(), slog.LevelError))
}

// The bridge replaces the logger but preserves the original stdout output.
func TestBridgeSlogToOTel(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	cfg := &config.Prest{Logger: base}
	cfg.Otel.ServiceName = "prestd-test"

	lp := sdklog.NewLoggerProvider()
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	bridgeSlogToOTel(cfg, lp)

	require.NotSame(t, base, cfg.Logger) // logger was replaced by the fanout
	cfg.Logger.Info("bridged-line", "k", "v")
	require.Contains(t, buf.String(), "bridged-line") // stdout JSON preserved
}

// With no base logger the bridge is a no-op and does not panic.
func TestBridgeSlogToOTel_nilLoggerNoop(t *testing.T) {
	cfg := &config.Prest{}
	lp := sdklog.NewLoggerProvider()
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	bridgeSlogToOTel(cfg, lp)
	require.Nil(t, cfg.Logger)
}

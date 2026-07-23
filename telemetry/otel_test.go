package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/prest/prest/v2/config"

	"github.com/stretchr/testify/require"
)

// Disabled (default): Init opens nothing and returns a callable no-op shutdown.
func TestInitDisabled(t *testing.T) {
	cfg := &config.Prest{}
	cfg.Otel.Enabled = false

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown(context.Background()))
}

// Nil config must not panic and behaves as disabled.
func TestInitNilConfig(t *testing.T) {
	shutdown, err := Init(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown(context.Background()))
}

// Enabled: providers are constructed without dialing a collector (the OTLP gRPC
// client connects lazily). Shutdown is exercised with a cancelled context so no
// live export is attempted.
func TestInitEnabledConstructsProviders(t *testing.T) {
	cfg := &config.Prest{}
	cfg.Otel.Enabled = true
	cfg.Otel.ServiceName = "prestd-test"
	cfg.Otel.Endpoint = "127.0.0.1:4317"
	cfg.Otel.Insecure = true
	cfg.Otel.SampleRatio = 1.0
	cfg.Otel.MetricsInterval = time.Minute

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = shutdown(ctx) // best-effort flush; no network assertion
}

// Exporter options reflect endpoint/insecure config without any network I/O.
func TestExporterOptions(t *testing.T) {
	t.Parallel()

	with := &config.Prest{}
	with.Otel.Endpoint = "collector:4317"
	with.Otel.Insecure = true
	require.Len(t, traceOptions(with), 2)
	require.Len(t, metricOptions(with), 2)

	empty := &config.Prest{}
	require.Empty(t, traceOptions(empty))
	require.Empty(t, metricOptions(empty))
}

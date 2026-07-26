package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// Defaults: telemetry off, sane product defaults, no outbound config.
func TestParseOtelDefaults(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))

	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)

	require.False(t, cfg.Otel.Enabled)
	require.Equal(t, "prestd", cfg.Otel.ServiceName)
	require.Equal(t, OtelProtocolGRPC, cfg.Otel.Protocol)
	require.Equal(t, 1.0, cfg.Otel.SampleRatio)
	require.Equal(t, 15*time.Second, cfg.Otel.MetricsInterval)
	require.False(t, cfg.Otel.Insecure)
	require.False(t, cfg.Otel.DBStatement)
	require.Empty(t, cfg.Otel.Endpoint)
}

// Env overrides use the PREST_OTEL_* prefix.
func TestParseOtelEnvOverride(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("PREST_OTEL_ENABLED", "true")
	t.Setenv("PREST_OTEL_ENDPOINT", "collector:4317")
	t.Setenv("PREST_OTEL_SAMPLE_RATIO", "0.25")
	t.Setenv("PREST_OTEL_METRICS_INTERVAL", "30s")
	t.Setenv("PREST_OTEL_INSECURE", "true")
	t.Setenv("PREST_OTEL_DB_STATEMENT", "true")

	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)

	require.True(t, cfg.Otel.Enabled)
	require.Equal(t, "collector:4317", cfg.Otel.Endpoint)
	require.Equal(t, 0.25, cfg.Otel.SampleRatio)
	require.Equal(t, 30*time.Second, cfg.Otel.MetricsInterval)
	require.True(t, cfg.Otel.Insecure)
	require.True(t, cfg.Otel.DBStatement)
}

// An unsupported protocol warns and falls back to grpc; empty defaults to grpc.
func TestParseOtelProtocolFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  bool
		in   string
	}{
		{"unsupported falls back", true, "http"},
		{"empty defaults", false, ""},
		{"grpc preserved", true, "grpc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := viper.New()
			if tc.set {
				v.Set("otel.protocol", tc.in)
			}
			cfg := &Prest{}
			parseOtelConfig(v, cfg)
			require.Equal(t, OtelProtocolGRPC, cfg.Otel.Protocol)
		})
	}
}

// sample_ratio is clamped to [0,1]; invalid interval falls back to default.
func TestParseOtelClampsAndFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ratio        float64
		interval     string
		wantRatio    float64
		wantInterval time.Duration
	}{
		{"above range clamps to 1", 2.0, "5s", 1.0, 5 * time.Second},
		{"below range clamps to 0", -1.0, "", 0.0, 15 * time.Second},
		{"in range preserved", 0.5, "1m", 0.5, time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := viper.New()
			v.Set("otel.sample_ratio", tc.ratio)
			if tc.interval != "" {
				v.Set("otel.metrics_interval", tc.interval)
			}
			cfg := &Prest{}
			parseOtelConfig(v, cfg)
			require.Equal(t, tc.wantRatio, cfg.Otel.SampleRatio)
			require.Equal(t, tc.wantInterval, cfg.Otel.MetricsInterval)
			// service name / protocol always default when unset
			require.Equal(t, "prestd", cfg.Otel.ServiceName)
			require.Equal(t, OtelProtocolGRPC, cfg.Otel.Protocol)
		})
	}
}

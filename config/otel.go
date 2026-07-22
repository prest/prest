package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	// OtelProtocolGRPC is the only exporter protocol supported in v1.
	OtelProtocolGRPC = "grpc"

	defaultOtelServiceName     = "prestd"
	defaultOtelMetricsInterval = 15 * time.Second
)

// OtelConf holds OpenTelemetry (OTLP push) settings. Telemetry is opt-in and
// disabled by default; when Enabled is false pREST performs no OTel setup and
// opens no outbound connections.
type OtelConf struct {
	Enabled         bool
	ServiceName     string
	Endpoint        string
	Protocol        string
	SampleRatio     float64
	MetricsInterval time.Duration
	Insecure        bool
	// DBStatement, when true, allows the SQL statement text to be recorded on
	// DB spans. Default false to avoid leaking user data/secrets.
	DBStatement bool
}

// parseOtelConfig reads the [otel] section. Env overrides use the PREST_OTEL_*
// prefix; when a key is left at its zero value the OTel SDK/exporter still reads
// standard OTEL_* environment variables at init time.
func parseOtelConfig(v *viper.Viper, cfg *Prest) {
	o := &cfg.Otel
	o.Enabled = v.GetBool("otel.enabled")

	o.ServiceName = strings.TrimSpace(v.GetString("otel.service_name"))
	if o.ServiceName == "" {
		o.ServiceName = defaultOtelServiceName
	}

	o.Endpoint = strings.TrimSpace(v.GetString("otel.endpoint"))

	o.Protocol = strings.ToLower(strings.TrimSpace(v.GetString("otel.protocol")))
	if o.Protocol == "" {
		o.Protocol = OtelProtocolGRPC
	}

	o.SampleRatio = v.GetFloat64("otel.sample_ratio")
	if o.SampleRatio < 0 {
		o.SampleRatio = 0
	}
	if o.SampleRatio > 1 {
		o.SampleRatio = 1
	}

	o.MetricsInterval = v.GetDuration("otel.metrics_interval")
	if o.MetricsInterval <= 0 {
		o.MetricsInterval = defaultOtelMetricsInterval
	}

	o.Insecure = v.GetBool("otel.insecure")
	o.DBStatement = v.GetBool("otel.db_statement")
}

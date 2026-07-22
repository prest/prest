// Package telemetry sets up OpenTelemetry trace and metric providers that
// export over OTLP (gRPC) push. It exposes no HTTP endpoint: telemetry is
// pushed to a collector configured by the operator. Setup is opt-in via
// [otel] config and fails closed — when disabled it performs no work and
// opens no outbound connections.
package telemetry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/helpers"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ShutdownFunc flushes and shuts down the telemetry providers. It is safe to
// call even when telemetry is disabled (no-op).
type ShutdownFunc func(context.Context) error

func noopShutdown(context.Context) error { return nil }

// Init configures global OpenTelemetry providers and the W3C trace-context
// propagator from cfg. It returns a shutdown function that flushes exporters.
//
// When cfg.Otel.Enabled is false, Init returns a no-op shutdown and nil error
// without registering providers or dialing any collector.
//
// Exporter construction failures are logged as warnings and telemetry is
// disabled (warn + disable): Init returns a no-op shutdown and nil error so the
// server still starts.
func Init(ctx context.Context, cfg *config.Prest) (ShutdownFunc, error) {
	if cfg == nil || !cfg.Otel.Enabled {
		return noopShutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Otel.ServiceName),
			semconv.ServiceVersion(helpers.PrestReleaseVersion()),
		),
	)
	if err != nil {
		slog.Warn("otel: building resource failed, telemetry disabled", "err", err)
		return noopShutdown, nil
	}

	traceExp, err := otlptracegrpc.New(ctx, traceOptions(cfg)...)
	if err != nil {
		slog.Warn("otel: trace exporter setup failed, telemetry disabled", "err", err)
		return noopShutdown, nil
	}

	metricExp, err := otlpmetricgrpc.New(ctx, metricOptions(cfg)...)
	if err != nil {
		slog.Warn("otel: metric exporter setup failed, telemetry disabled", "err", err)
		// best-effort flush of the already-built trace exporter
		_ = traceExp.Shutdown(ctx)
		return noopShutdown, nil
	}

	logExp, err := otlploggrpc.New(ctx, logOptions(cfg)...)
	if err != nil {
		slog.Warn("otel: log exporter setup failed, telemetry disabled", "err", err)
		_ = traceExp.Shutdown(ctx)
		_ = metricExp.Shutdown(ctx)
		return noopShutdown, nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.Otel.SampleRatio))),
		sdktrace.WithBatcher(traceExp),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(cfg.Otel.MetricsInterval))),
	)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	logglobal.SetLoggerProvider(lp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Bridge slog -> OTel logs: keep the existing stdout JSON handler and also
	// emit records to the OTLP log pipeline, correlated to the active span.
	bridgeSlogToOTel(cfg, lp)

	slog.Info("otel: telemetry enabled",
		"service", cfg.Otel.ServiceName,
		"endpoint", cfg.Otel.Endpoint,
		"sample_ratio", cfg.Otel.SampleRatio)

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

// bridgeSlogToOTel reconfigures the process logger to fan out to the existing
// handler (stdout JSON) and an OTel logs bridge handler. It is a no-op when no
// base logger is configured.
func bridgeSlogToOTel(cfg *config.Prest, lp *sdklog.LoggerProvider) {
	if cfg.Logger == nil {
		return
	}
	otelHandler := otelslog.NewHandler(cfg.Otel.ServiceName, otelslog.WithLoggerProvider(lp))
	logger := slog.New(newFanoutHandler(cfg.Logger.Handler(), otelHandler))
	cfg.Logger = logger
	slog.SetDefault(logger)
}

func logOptions(cfg *config.Prest) []otlploggrpc.Option {
	opts := []otlploggrpc.Option{}
	if cfg.Otel.Endpoint != "" {
		opts = append(opts, otlploggrpc.WithEndpoint(cfg.Otel.Endpoint))
	}
	if cfg.Otel.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	return opts
}

func traceOptions(cfg *config.Prest) []otlptracegrpc.Option {
	opts := []otlptracegrpc.Option{}
	if cfg.Otel.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Otel.Endpoint))
	}
	if cfg.Otel.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	return opts
}

func metricOptions(cfg *config.Prest) []otlpmetricgrpc.Option {
	opts := []otlpmetricgrpc.Option{}
	if cfg.Otel.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Otel.Endpoint))
	}
	if cfg.Otel.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	return opts
}

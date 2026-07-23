package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log/slog"
	"sync"

	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// otelDrivers caches one instrumented driver name per distinct telemetry option
// set (keyed by DBStatement), so a manager always receives a driver matching its
// own configuration regardless of construction order. database/sql drivers are
// process-global, so each distinct configuration is registered at most once.
var (
	otelDriverMu    sync.Mutex
	otelDriverNames = map[bool]string{}
)

// otelManagerOptions returns connection.Manager options that route the pool
// through an OpenTelemetry-instrumented driver and publish DB pool metrics,
// when cfg.Otel.Enabled is set. On registration failure it logs a warning and
// returns nil, so the pool falls back to the default driver rather than blocking
// startup.
//
// DB spans are tagged with db.namespace using the database alias carried in the
// request context (pctx.DBNameKey). The raw SQL statement is recorded only when
// cfg.Otel.DBStatement is true (default false), to avoid leaking user data.
func otelManagerOptions(cfg *config.Prest) []connection.ManagerOption {
	if cfg == nil || !cfg.Otel.Enabled {
		return nil
	}

	driverName, err := instrumentedDriver(cfg.Otel.DBStatement)
	if err != nil {
		slog.Warn("otel: registering instrumented db driver failed, db telemetry disabled", "err", err)
		return nil
	}

	return []connection.ManagerOption{
		connection.WithDriverName(driverName),
		connection.WithAfterConnect(func(db *sql.DB) {
			// Pool stats metrics are best-effort; a failure must not break setup.
			_, _ = otelsql.RegisterDBStatsMetrics(db,
				otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
		}),
	}
}

// instrumentedDriver returns the otelsql-wrapped "postgres" driver name for the
// given DBStatement setting, registering (and caching) it on first use.
func instrumentedDriver(dbStatement bool) (string, error) {
	otelDriverMu.Lock()
	defer otelDriverMu.Unlock()

	if name, ok := otelDriverNames[dbStatement]; ok {
		return name, nil
	}

	name, err := otelsql.Register("postgres",
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{DisableQuery: !dbStatement}),
		otelsql.WithAttributesGetter(dbAliasAttributes),
	)
	if err != nil {
		return "", err
	}
	otelDriverNames[dbStatement] = name
	return name, nil
}

// dbAliasAttributes tags each DB span with the database alias from the request
// context so operators can slice DB telemetry per configured database.
func dbAliasAttributes(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) []attribute.KeyValue {
	if name, ok := ctx.Value(pctx.DBNameKey).(string); ok && name != "" {
		return []attribute.KeyValue{semconv.DBNamespace(name)}
	}
	return nil
}

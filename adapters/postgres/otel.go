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

var (
	otelRegisterOnce sync.Once
	otelDriverName   string
	otelRegisterErr  error
)

// otelManagerOptions returns connection.Manager options that route the pool
// through an OpenTelemetry-instrumented driver and publish DB pool metrics,
// when cfg.Otel.Enabled is set. The lib/pq "postgres" driver is wrapped once
// per process (database/sql drivers are process-global). On registration
// failure it logs a warning and returns nil, so the pool falls back to the
// default driver rather than blocking startup.
//
// DB spans are tagged with db.namespace using the database alias carried in the
// request context (pctx.DBNameKey). The raw SQL statement is recorded only when
// cfg.Otel.DBStatement is true (default false), to avoid leaking user data.
func otelManagerOptions(cfg *config.Prest) []connection.ManagerOption {
	if cfg == nil || !cfg.Otel.Enabled {
		return nil
	}

	otelRegisterOnce.Do(func() {
		otelDriverName, otelRegisterErr = otelsql.Register("postgres",
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
			otelsql.WithSpanOptions(otelsql.SpanOptions{DisableQuery: !cfg.Otel.DBStatement}),
			otelsql.WithAttributesGetter(dbAliasAttributes),
		)
		if otelRegisterErr != nil {
			slog.Warn("otel: registering instrumented db driver failed, db telemetry disabled", "err", otelRegisterErr)
		}
	})
	if otelRegisterErr != nil {
		return nil
	}

	return []connection.ManagerOption{
		connection.WithDriverName(otelDriverName),
		connection.WithAfterConnect(func(db *sql.DB) {
			// Pool stats metrics are best-effort; a failure must not break setup.
			_, _ = otelsql.RegisterDBStatsMetrics(db,
				otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
		}),
	}
}

// dbAliasAttributes tags each DB span with the database alias from the request
// context so operators can slice DB telemetry per configured database.
func dbAliasAttributes(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) []attribute.KeyValue {
	if name, ok := ctx.Value(pctx.DBNameKey).(string); ok && name != "" {
		return []attribute.KeyValue{semconv.DBNamespace(name)}
	}
	return nil
}

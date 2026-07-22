package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// RegisterOTelDriver wraps the lib/pq "postgres" driver with OpenTelemetry
// instrumentation and points the connection pool at the wrapped driver. It also
// registers a post-connect hook that publishes DB pool metrics for every pooled
// database. Call once at startup, after telemetry providers are initialized and
// before any connection is opened.
//
// DB spans are tagged with db.namespace using the database alias carried in the
// request context (pctx.DBNameKey). The raw SQL statement is recorded only when
// cfg.Otel.DBStatement is true (default false), to avoid leaking user data.
func RegisterOTelDriver(cfg *config.Prest) error {
	opts := []otelsql.Option{
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			DisableQuery: !cfg.Otel.DBStatement,
		}),
		otelsql.WithAttributesGetter(dbAliasAttributes),
	}

	driverName, err := otelsql.Register("postgres", opts...)
	if err != nil {
		return err
	}

	connection.SetDriverName(driverName)
	connection.SetAfterConnect(func(db *sql.DB) {
		// Pool stats metrics are best-effort; a failure here must not break
		// connection setup.
		_, _ = otelsql.RegisterDBStatsMetrics(db,
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
	})
	return nil
}

// dbAliasAttributes tags each DB span with the database alias from the request
// context so operators can slice DB telemetry per configured database.
func dbAliasAttributes(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) []attribute.KeyValue {
	if name, ok := ctx.Value(pctx.DBNameKey).(string); ok && name != "" {
		return []attribute.KeyValue{semconv.DBNamespace(name)}
	}
	return nil
}

package timescaledb

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/internal/ident"
)

// Adapter wraps the postgres adapter and adds TimescaleDB-specific enhancements.
// It embeds the adapters.Adapter interface to automatically delegate all methods
// except those that are explicitly overridden.
type Adapter struct {
	adapters.Adapter
}

// New creates a TimescaleDB adapter.
// TimescaleDB is wire-compatible with PostgreSQL, so we wrap the postgres adapter
// and override only TimescaleDB-specific methods like schema filtering.
func New(cfg *config.Prest) adapters.Adapter {
	return &Adapter{
		Adapter: postgres.New(cfg),
	}
}

// SchemaClause overrides the postgres implementation to filter system schemas.
// TimescaleDB creates _timescaledb_* system schemas that clutter the schema list.
// This method hides them by default unless _include_system_schemas=true is set.
func (a *Adapter) SchemaClause(req *http.Request) (query string, hasCount bool) {
	// Get base query from embedded postgres adapter
	query, hasCount = a.Adapter.SchemaClause(req)

	// Add TimescaleDB-specific filtering unless opt-in flag set
	if req.URL.Query().Get("_include_system_schemas") != "true" {
		query = fmt.Sprint(query, " WHERE schema_name NOT LIKE '_timescaledb_%'")
	}
	return
}

// TimeBucketClause generates a GROUP BY clause for TimescaleDB time_bucket aggregations.
// Parses _time_bucket=interval[,column] where interval is 5m, 1h, 1d, etc.
// This is a TimescaleDB-specific operator; the base postgres adapter does not support it.
// Example: _time_bucket=1h -> GROUP BY time_bucket('1 hour', time)
// Example: _time_bucket=1h,created_at -> GROUP BY time_bucket('1 hour', created_at)
func (a *Adapter) TimeBucketClause(req *http.Request) (groupBySQL string, err error) {
	queries := req.URL.Query()
	timeBucketParam := queries.Get("_time_bucket")
	if timeBucketParam == "" {
		return
	}

	// Parse interval and optional column: "1h" or "1h,created_at"
	parts := strings.Split(strings.TrimSpace(timeBucketParam), ",")
	interval := strings.TrimSpace(parts[0])
	timeColumn := "time"
	if len(parts) > 1 {
		timeColumn = strings.TrimSpace(parts[1])
	}

	// Validate interval format
	sqlInterval, ok := timeBucketIntervalMap[interval]
	if !ok {
		return "", fmt.Errorf("invalid time_bucket interval: %s (supported: 5m, 15m, 1h, 6h, 1d, 7d, 30d, 1y)", interval)
	}

	// Validate column name
	if !ident.IsValid(timeColumn) {
		return "", fmt.Errorf("invalid column name: %s", timeColumn)
	}
	quotedColumn, _ := ident.Quote(timeColumn)

	// Generate GROUP BY clause
	groupBySQL = fmt.Sprintf("GROUP BY time_bucket('%s', %s)", sqlInterval, quotedColumn)
	return
}

// timeBucketIntervalMap maps user-friendly intervals to PostgreSQL interval syntax
// This is TimescaleDB-specific functionality
var timeBucketIntervalMap = map[string]string{
	"5m":  "5 minutes",
	"15m": "15 minutes",
	"1h":  "1 hour",
	"6h":  "6 hours",
	"1d":  "1 day",
	"7d":  "7 days",
	"30d": "30 days",
	"1y":  "1 year",
}

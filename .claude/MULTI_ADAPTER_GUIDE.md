# Multi-Adapter Guide: Running Multiple SQL Databases in One pREST Instance

## Overview

pREST v2.3+ supports running **multiple SQL databases simultaneously** behind a single REST API. Each database can use a different SQL engine (Postgres, TimescaleDB, MySQL, SQLite, etc.), and requests are automatically routed to the correct adapter based on the database name in the URL.

### Example Setup

```
┌─────────────────────────────────────────┐
│        Single pREST API Instance        │
│         (port 3000)                     │
└────────────────────────────────────────┘
           ↓ GET /{db}/...
    ┌──────┴──────┐
    ↓             ↓
┌─────────┐   ┌──────────────┐
│Postgres │   │ TimescaleDB  │
│(app_db) │   │  (metrics)   │
└─────────┘   └──────────────┘
```

### Request Format

```
GET /postgres/public/users           → queries Postgres database
GET /timescaledb/public/metrics      → queries TimescaleDB database
GET /warehouse/analytics/sales       → queries MySQL database
```

## Architecture

### 1. Adapter Registry

The **adapter registry** is the core of multi-database support. It maps database aliases to adapter instances:

```go
registry := adapters.NewRegistry()
registry.Register("postgres", postgresAdapter)
registry.Register("timescaledb", timescaledbAdapter)
registry.Register("warehouse", mysqlAdapter)

// At request time:
adapter, err := registry.Get("timescaledb")  // Get TimescaleDB adapter
```

### 2. Adapter Selection Middleware

The `AdapterSelectorMiddleware` runs early in the request pipeline:

1. Extracts database name from URL path (`/{database}/...`)
2. Looks up adapter in registry
3. Attaches adapter to request context
4. Passes request to next handler

Handlers retrieve the adapter from context and use it for that database.

### 3. Per-Database Adapter Detection

When pREST starts up, for each configured database it:

1. Attempts to connect as **TimescaleDB** (checks for `timescaledb` extension)
2. If that fails, attempts to connect as **Postgres**
3. Future: Attempts MySQL, SQLite, etc.

This allows automatic detection without explicit configuration.

## Configuration

### TOML Format

```toml
[[databases]]
alias = "postgres"
host = "localhost"
port = 5432
user = "prest"
pass = "password"
database = "app_db"
maxopenconn = 10
maxidleconn = 2

[[databases]]
alias = "timescaledb"
host = "timescale.example.com"
port = 5432
user = "prest"
pass = "password"
database = "metrics"

[[databases]]
alias = "warehouse"
url = "mysql://user:pass@mysql.example.com/data"
```

### Environment Variables

```bash
# Define databases via environment
export PREST_DATABASES_0_ALIAS=postgres
export PREST_DATABASES_0_HOST=localhost
export PREST_DATABASES_0_PASS=secret

export PREST_DATABASES_1_ALIAS=timescaledb
export PREST_DATABASES_1_URL=postgres://user:pass@ts-host/metrics
```

### See Also

Full examples: `[.claude/examples/multi-database-config.toml](./examples/multi-database-config.toml)`

## How Requests Are Routed

### 1. Request Arrives

```
GET /timescaledb/public/sensor_data?_page_size=10
```

### 2. AdapterSelectorMiddleware

Extracts database name: `"timescaledb"`

### 3. Registry Lookup

```go
adapter, err := registry.Get("timescaledb")
// Returns the connected TimescaleDB adapter
```

### 4. Context Attachment

```go
ctx := context.WithValue(r.Context(), pctx.AdapterKey, adapter)
r = r.WithContext(ctx)
```

### 5. Handler Execution

```go
adapter := GetAdapterForRequest(r, defaultAdapter)
// Returns the TimescaleDB adapter for this request
rows := adapter.Query("SELECT * FROM sensor_data...")
```

## Usage Examples

### Single Database (Backward Compatible)

If you don't configure `[[databases]]`, pREST auto-detects Postgres/TimescaleDB:

```toml
# No [[databases]] section needed
# pREST will use PGHost, PGPort, PGUser, etc.
```

Requests work as before:

```bash
curl http://localhost:3000/prest-test/public/users
```

### Multiple Databases

Configure `[[databases]]` entries:

```toml
[[databases]]
alias = "app"
host = "db1.example.com"
database = "app_db"

[[databases]]
alias = "metrics"
url = "postgres://user:pass@ts1.example.com/metrics"
```

Requests include database name:

```bash
# Query app database
curl http://localhost:3000/app/public/users

# Query metrics database (TimescaleDB)
curl http://localhost:3000/metrics/public/events

# Group by time_bucket (TimescaleDB specific)
curl "http://localhost:3000/metrics/public/events?_groupby=time_bucket(%271%20hour%27,timestamp)"
```

### Per-Database SSL Configuration

```toml
[[databases]]
alias = "secure_db"
host = "secure.internal"
user = "prest"
pass = "password"
database = "app"

[databases.ssl]
mode = "require"
cert = "/etc/prest/certs/client.crt"
key = "/etc/prest/certs/client.key"
rootcert = "/etc/prest/certs/ca.crt"
```

## Handler Implementation

### Getting the Adapter

Handlers should use `GetAdapterForRequest()` to get the correct adapter:

```go
func (h *MyHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // Get the adapter for this request's database
    // (from context if multi-DB, or default if single-DB)
    adapter := GetAdapterForRequest(r, h.defaultAdapter)
    
    // Use the adapter
    rows := adapter.Query("SELECT * FROM table")
}
```

### Example: Table Handler

```go
type TableHandler struct {
    executor adapters.QueryExecutor
    db       adapters.DatabaseRegistry
    registry adapters.Registry  // New: multi-database support
    singleDB bool
}

func (h *TableHandler) Show(w http.ResponseWriter, r *http.Request) {
    vars := pathVars(r)
    database := vars["database"]
    
    // For multi-DB: get full adapter from registry
    if h.registry != nil && database != "" {
        adapter, err := h.registry.Get(database)
        if err != nil {
            jsonError(w, "database not found", http.StatusNotFound)
            return
        }
        // Use adapter for this database
        // ...
    } else {
        // Single-DB mode: use injected executor
        // ...
    }
}
```

## Database-Specific Features

### TimescaleDB

Time bucketing operator:

```bash
curl "http://localhost:3000/timescaledb/public/metrics?_groupby=time_bucket(%275%20minutes%27,time)"
```

Continuous aggregates:

```bash
# Query a materialized view (continuous aggregate)
curl http://localhost:3000/timescaledb/public/hourly_summary
```

### Postgres

All standard SQL operations work on any database alias.

### Future: MySQL, SQLite, etc.

As new adapters are added:

```bash
curl http://localhost:3000/warehouse/sales/orders      # MySQL
curl http://localhost:3000/analytics/queries/reports   # SQLite
```

## Performance Considerations

### Connection Pooling

Each database gets its own connection pool (configured per database):

```toml
[[databases]]
alias = "timescaledb"
maxopenconn = 25  # Connection pool size
maxidleconn = 5   # Idle connections to keep open
```

### Startup Time

pREST connects to all configured databases at startup. If a database is unreachable:

- A warning is logged
- The database is not registered
- Other databases continue to work
- Requests to the unreachable database return 404

### Memory Usage

Memory usage scales with:
- Number of databases
- Connection pool size per database
- Query result set sizes

Monitor memory usage and adjust pool sizes accordingly.

## Troubleshooting

### "Database not found" Error

```
GET /unknown_db/public/users → 404 not found
```

**Cause:** Database alias is not configured or not connected.

**Solution:** Check config and ensure database is in `[[databases]]` section.

### Connection Errors at Startup

```
warn: failed to connect to database | alias=timescaledb
```

**Cause:** Database is unreachable or credentials are wrong.

**Solution:** 
- Check network connectivity
- Verify credentials
- Ensure database server is running

### Adapter Detection Issues

**Symptom:** TimescaleDB database detected as Postgres.

**Reason:** This is normal - Postgres adapter works for both (wire-compatible).

**If you need TimescaleDB-specific features:**
- Use `/_QUERIES` templates for `time_bucket()`, `continuous aggregates`, etc.
- These work automatically on TimescaleDB, are ignored on Postgres

## API Reference

### adapter_helper.go Functions

```go
// Get adapter for current request (context or default)
adapter := GetAdapterForRequest(r, defaultAdapter)

// Get adapter from registry by database name
adapter, err := GetAdapterFromRegistry(registry, "timescaledb")
if err != nil {
    // Database not found or registry is nil
}
```

### Registry Interface

```go
type Registry interface {
    Register(alias string, adapter Adapter) error
    Get(alias string) (Adapter, error)
    GetAll() []string
    Aliases() []string
    IsRegistered(alias string) bool
}
```

## Migration from Single to Multi-Database

### Step 1: Add Database Configuration

```toml
# Keep your existing PGHost, PGPort, etc. for backward compatibility
PGHost = "localhost"
PGPort = 5432

# Add [[databases]] section for multi-database
[[databases]]
alias = "primary"
host = "localhost"
port = 5432
user = "prest"
database = "app"
```

### Step 2: Update Requests

Old (single-database):
```bash
curl http://localhost:3000/prest-test/public/users
```

New (multi-database):
```bash
curl http://localhost:3000/primary/public/users
```

### Step 3: Gradual Migration

Keep both formats working during transition:
- Single-database requests still work (use default adapter)
- Multi-database requests work (use registered adapters)

## Best Practices

1. **Use descriptive aliases:** `primary`, `analytics`, `cache` instead of `db1`, `db2`
2. **Group by purpose:** Separate application, metrics, reporting, cache databases
3. **Monitor connections:** Each database needs connection slots; configure pool sizes appropriately
4. **Use environment variables:** Keep secrets out of config files
5. **Test failover:** Verify behavior when a database is unreachable
6. **Document aliases:** Keep a runbook of which database does what

## See Also

- **CLAUDE.md** - Developer guidelines for multi-adapter architecture
- **examples/multi-database-config.toml** - Configuration examples
- **adapters/registry.go** - Registry implementation
- **middlewares/adapter_selector.go** - Request routing middleware

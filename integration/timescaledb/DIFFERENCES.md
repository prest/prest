# TimescaleDB vs stock Postgres (pREST)

pREST v2.3+ provides **first-class TimescaleDB support** via automatic adapter detection.
At startup, pREST detects whether connected to TimescaleDB and selects the appropriate adapter.
TimescaleDB is wire-compatible with PostgreSQL, so the postgres adapter handles all CRUD;
TimescaleDB-specific features (time_bucket, continuous aggregates) are supported at the query level.

| Area | Stock Postgres | TimescaleDB impact on pREST |
|------|----------------|-----------------------------|
| Image / init | `postgres:18` | `timescale/timescaledb:latest-pg18`; pREST detects extension automatically |
| Adapter selection | Uses `adapters/postgres` | Auto-detected via `SELECT exists(SELECT 1 FROM pg_extension WHERE extname='timescaledb')` |
| Catalog | Plain tables/views | Hypertables appear as regular tables; chunks and `_timescaledb_*` catalogs also visible |
| DDL | Standard SQL | Hypertable conversion (`create_hypertable`), policies — use `/_QUERIES` custom SQL |
| Time-series operators | N/A | `_groupby=time_bucket('interval', column)` supported for time-based aggregation |
| Continuous aggregates | N/A | Materialized views appear as queryable tables; create via `/_QUERIES` custom SQL |
| Wire / driver | `lib/pq` + postgres adapter | Same `lib/pq` + postgres adapter (Timescale is wire-compatible) |
| System schemas | `pg_catalog` / `information_schema` | Extra `_timescaledb_*` schemas visible; can be filtered with ACL later |
| Compose | Multi-service (auth, multicluster, queries) | Lean stack for suites + specific tests; other flavors stay on the Postgres job |

## Shared suites vs Timescale E2E

Wire-compatible shared suites live under `integration/suites/` and run on the
**Postgres** integration workflow (`make test-integration-postgres`).

The TimescaleDB workflow (`test-integration-timescaledb.yml` /
`make test-integration-timescaledb`) runs **only**
`./integration/timescaledb/...` against this compose — engine-specific E2E
(extension detection, hypertable operations, time_bucket queries).

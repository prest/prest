# TimescaleDB vs stock Postgres (pREST)

TimescaleDB is a PostgreSQL-wire engine. pREST v1 talks to it through the
existing `adapters/postgres` stack — no separate adapter.

| Area | Stock Postgres | TimescaleDB impact on pREST |
|------|----------------|-----------------------------|
| Image / init | `postgres:18` | `timescale/timescaledb:latest-pg18`; require `CREATE EXTENSION IF NOT EXISTS timescaledb` |
| Catalog | Plain tables/views | Hypertables, chunk children, and Timescale catalogs may appear in `/tables` / `/schemas` |
| DDL | Standard SQL | Hypertable conversion (`create_hypertable`), policies — covered by Timescale-specific tests |
| Wire / driver | `lib/pq` + postgres adapter | Same |
| System schemas | `pg_catalog` / `information_schema` | Extra `_timescaledb_*` schemas; ACL/`access_confine` may need allowlists later |
| Compose | Multi-service (auth, multicluster, queries) | Lean stack for suites + specific tests; other flavors stay on the Postgres job |

## Shared suites vs Timescale E2E

Wire-compatible shared suites live under `integration/suites/` and run on the
**Postgres** integration workflow (`make test-integration-postgres`).

The TimescaleDB workflow (`test-integration-timescaledb.yml` /
`make test-integration-timescaledb`) runs **only**
`./integration/timescaledb/...` against this compose — engine-specific E2E
(extension, hypertable, catalog quirks).

# SQL database support — Timescale reference

TimescaleDB is a **Postgres-wire variant**: no new adapter for v1.

## Layout used

```text
integration/timescaledb/
  docker-compose.yml    # timescale/timescaledb:latest-pg18 + prestd + tests
  db-init.sh            # testdata/db-init.sh + CREATE EXTENSION + hypertable seed
  DIFFERENCES.md
  controllers/          # Timescale-specific HTTP tests
```

## CI

- Workflow: `.github/workflows/test-integration-timescaledb.yml`
- Make: `make test-integration-timescaledb`
- Packages (Timescale-only E2E):

```bash
go test ./integration/timescaledb/...
```

Shared `suites/` stay on the Postgres integration workflow.
Auth / multicluster / queries stacks remain on Postgres compose.

## Thinking checklist applied

1. Classify → wire variant → reuse `adapters/postgres`
2. Document differences (extension, hypertables, extra schemas)
3. Lean compose (no multicluster for v1)
4. Timescale-specific hypertable E2E under `integration/timescaledb/`
5. Dedicated workflow + Makefile target (parallel with Postgres)

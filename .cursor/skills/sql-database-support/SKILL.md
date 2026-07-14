---
name: sql-database-support
description: >-
  Guides classifying, gap-analyzing, and scaffolding support for a new SQL
  database in pREST (wire-compatible variants like TimescaleDB or new dialects).
  Use when adding database support, creating integration/<db>/, DIFFERENCES.md,
  per-DB docker-compose or GitHub workflows, or planning a new adapters/<engine>.
---

# Adding SQL Database Support

Use this skill to **think through and start** support for a SQL engine — not to
implement a full product surface in one pass.

Reference first engine: `integration/timescaledb/` (+ `DIFFERENCES.md`).
Timescale CI: `.github/workflows/test-integration-timescaledb.yml` →
`make test-integration-timescaledb` → `./integration/timescaledb/...` only.
Integration comment style: skill `prest-integration-tests`.

## 1. Classify the engine

| Kind | Examples | Implication |
|------|----------|-------------|
| Postgres-wire variant | TimescaleDB, Citus | Reuse `adapters/postgres`; focus on compose, seed, catalog quirks, specific tests |
| New dialect | MySQL, SQLite, … | New `adapters/<engine>` implementing `adapters.Adapter` ports; config/driver selection in `app/app.go` (today hardcodes `postgres.New`) |

Do not pretend a new dialect is “just compose” — ports (catalog SQL, quoting,
RETURNING, types) must be designed before CI greenwashing.

## 2. Gap analysis → DIFFERENCES.md

Create `integration/<db>/DIFFERENCES.md` covering at least:

- Image / init (extensions, required SQL)
- Catalog / listing behaviour vs stock Postgres
- DDL features unique to the engine
- Wire / driver / adapter choice
- System schemas and ACL/`access_confine` implications
- Which compose flavours this DB job will own vs leave on Postgres

## 3. Integration layout contract

```text
integration/
  helpers/ testutils/ suites/   # shared
  <db>/
    docker-compose.yml
    DIFFERENCES.md
    ... engine-specific tests ...
```

- One compose file **per DB**. Never fold a new engine into
  `integration/postgres/docker-compose.yml`.
- Makefile: `test-integration-<db>`.
- CI: **dedicated GitHub Actions workflow**
  (`.github/workflows/test-integration-<db>.yml`), parallel with Postgres —
  not a job buried only inside the Postgres workflow.
- Package scope for a DB workflow:
  - **Engine-specific E2E:** `./integration/<db>/...` only (Timescale pattern).
  - Shared wire-compatible `suites/` stay on the Postgres integration workflow
    unless you intentionally add a separate compat job.

## 4. Test policy

1. **Smoke / specific** — extension present, one engine-native feature
   (e.g. Timescale hypertable read) under `integration/<db>/`.
2. **Compat** — shared `suites/` run on Postgres compose; add a DB-specific
   compat job only if you need to gate an engine against that suite.
3. Document every request per `prest-integration-tests`.

## 5. Safety

- Parameterized SQL only; no concatenating untrusted identifiers unchecked.
- Auth / ACL / JWT / secrets fail closed — never weaken for “compatibility”.
- Do not call `postgres.Load()` or open live DBs outside `integration/` and `cmd/`.

## Checklist (opening a new DB)

- [ ] Engine classified (wire variant vs new dialect)
- [ ] `integration/<db>/DIFFERENCES.md` written
- [ ] `integration/<db>/docker-compose.yml` + seed/init
- [ ] Specific smoke + feature tests under `integration/<db>/`
- [ ] `make test-integration-<db>` target (compose runs `./integration/<db>/...`)
- [ ] Dedicated workflow `.github/workflows/test-integration-<db>.yml`
- [ ] Workflow does not run other DBs’ packages
- [ ] Cross-link from this skill / `prest-integration-tests` as needed

## Examples

See [examples.md](examples.md) for the TimescaleDB path as the reference skeleton.

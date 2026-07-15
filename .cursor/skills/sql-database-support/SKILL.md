---
name: sql-database-support
description: >-
  Guides classifying, gap-analyzing, and scaffolding support for a new SQL
  database in pREST (wire-compatible variants like TimescaleDB or new dialects).
  Use when adding database support, creating integration/<db>/, DIFFERENCES.md,
  adapters/<engine>, per-DB docker-compose or GitHub workflows, or planning
  where config/app wiring must change.
---

# Adding SQL Database Support

MUST invariants: `.cursor/rules/sql-database-support.mdc`. Compose/workflow
package ownership: `.cursor/rules/integration-layout.mdc`. This skill is the
full analysis → where-to-change → scaffold playbook.

Use this to **think through and start** support for a SQL engine — not to ship
a full product surface in one pass.

Reference engine: `integration/timescaledb/` (+ `DIFFERENCES.md`). See
[examples.md](examples.md). Integration request style: skill `prest-integration-tests`.

## Phase A — Classify

Decide **before** scaffolding CI or adapters.

| Kind | Examples | Implication |
|------|----------|-------------|
| Postgres-wire variant | TimescaleDB, Citus | Reuse `adapters/postgres`; compose, seed, catalog quirks, specific tests |
| New dialect | MySQL, SQLite, … | New `adapters/<engine>` implementing `adapters.Adapter`; driver selection in `app/app.go` |

### Decision criteria

Ask whether these match stock Postgres:

- Wire protocol / driver (`lib/pq` vs another)
- SQL dialect (quoting, types, `RETURNING`, upserts)
- Catalog SQL used by pREST listing endpoints
- Identifier quoting and reserved words

If protocol **and** catalog SQL can stay on the postgres adapter with only
seed/extension quirks → **wire variant**. If ports must emit different SQL or
use another driver → **new dialect**. Do not greenwash a dialect as compose-only.

## Phase B — Gap analysis → DIFFERENCES.md

Write `integration/<db>/DIFFERENCES.md` **before** Make/CI targets that claim
support. Answer every item below (N/A with reason is fine).

### Worksheet

1. **Runtime image / init** — image tag; extensions; required init SQL
2. **Connection / DSN / driver** — same as Postgres or different?
3. **Identifier quoting / reserved words** — impacts builders and path params?
4. **Catalog listing** — `/databases`, `/schemas`, `/tables`, `/columns` vs stock Postgres (extra relation kinds, schemas, system catalogs)
5. **DDL / engine-native features** — hypertables, policies, etc.; which need specific tests?
6. **System schemas and ACL** — `access_confine` / allowlists implications
7. **Compose ownership** — which stacks this DB job owns vs leave on Postgres (auth, multicluster, queries)

Template shape: see `integration/timescaledb/DIFFERENCES.md`.

## Phase C — Where to change

### Wire-compatible variant (Timescale path)

| Add / change | Path |
|--------------|------|
| Differences doc | `integration/<db>/DIFFERENCES.md` |
| Compose + seed | `integration/<db>/docker-compose.yml`, init/`db-init.sh` |
| Engine E2E | `integration/<db>/...` only |
| Make target | `Makefile` → `test-integration-<db>` |
| CI | `.github/workflows/test-integration-<db>.yml` (parallel with Postgres) |

Do **not**: new `adapters/<engine>`; fold into `integration/postgres/docker-compose.yml`.

### New dialect (future MySQL / SQLite path)

| Add / change | Path |
|--------------|------|
| Ports (only if contract gaps) | `adapters/*.go` then `make mockgen` |
| Driven adapter | `adapters/<engine>/` implementing `adapters.Adapter` (+ connector/pinger as needed) |
| Composition root | `app/app.go` `New` / `EnsureAdapter` (today hardcodes `postgres.New`) |
| Config | `config/config.go` (+ fail-closed for credentials; see `config-resilience.mdc`) |
| Unit tests | co-located under `adapters/<engine>/` — TDD, ≥80% package coverage, no live DB (`unit-tests-tdd.mdc`, `adapter-unit-tests.mdc`) |
| Integration + CI | same layout as wire variant under `integration/<db>/` |

Do **not**: import the engine from `controllers/`, `middlewares/`, or `router/`
(hexagonal: `hexagonal-architecture.mdc`).

### Layout reminder

```text
integration/
  helpers/ testutils/ suites/   # shared; suites stay on Postgres job
  <db>/
    docker-compose.yml
    DIFFERENCES.md
    ... engine-specific tests ...
```

Details: `integration-layout.mdc`.

## Phase D — Test policy

1. **Smoke / specific** — extension present, one engine-native feature under `integration/<db>/`.
2. **Compat** — shared `suites/` run on Postgres compose; add a DB-specific compat job only if you intentionally gate that engine against the suite.
3. Document every request per skill `prest-integration-tests` / `integration-tests.mdc`.

## Safety

- Parameterized SQL only; no concatenating untrusted identifiers unchecked.
- Auth / ACL / JWT / secrets fail closed — never weaken for “compatibility”.
- Do not call `postgres.Load()` or open live DBs outside `integration/` and `cmd/`.

## Checklist (opening a new DB)

- [ ] Engine classified (wire variant vs new dialect)
- [ ] Gap worksheet answered; `integration/<db>/DIFFERENCES.md` written
- [ ] Where-matrix paths identified for this kind
- [ ] `integration/<db>/docker-compose.yml` + seed/init
- [ ] Specific smoke + feature tests under `integration/<db>/`
- [ ] `make test-integration-<db>` (compose runs `./integration/<db>/...` only)
- [ ] Dedicated workflow `.github/workflows/test-integration-<db>.yml`
- [ ] Workflow does not run other DBs’ packages
- [ ] New dialect only: adapter + `app` selection + mocks + unit tests (≥80%, TDD)
- [ ] Cross-link docs/skills as needed

## Examples

[examples.md](examples.md) — TimescaleDB skeleton + new-dialect file stub list.

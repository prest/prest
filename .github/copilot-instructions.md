# Copilot Instructions for pREST Go Backend

These instructions guide AI-generated changes in this repository.

## Project Scope

- Repository: `github.com/prest/prest`
- Language: Go (`go 1.26.0`, always check `go.mod`)
- Primary domain: PostgreSQL-backed REST API server, CLI, middleware, adapters, and plugins.
- Runtime entrypoint: `cmd/prestd/main.go`

## Core Engineering Principles

- Prefer small, focused changes over broad refactors.
- Preserve existing public behavior unless the task explicitly requires a change.
- Keep compatibility with Go 1.26.0.
- Favor readability and maintainability over clever abstractions.
- Do not introduce breaking changes to command flags, config keys, routes, or plugin behavior without explicit request.
- Apply SOLID when designing packages, ports, handlers, and adapters (see **SOLID Principles for Go** below).
- Keep the runtime stateless: no shared mutable application state between commands or requests (see **Go Style and Implementation Rules**).

## SOLID Principles for Go

Apply these on every change. They complement **Hexagonal Architecture** (below) — hexagonal defines layer boundaries; SOLID defines how types and dependencies inside those layers are shaped.

### S — Single Responsibility

One type or package should have one reason to change.

**pREST mapping:** `AuthHandler` handles authentication only; `CRUDHandler` handles table CRUD; `adapters/postgres/internal/connection` handles pooling only. Cross-cutting concerns (JWT, ACL, cache) belong in `middlewares/`, not inside handlers.

```go
// GOOD: focused handler
type AuthHandler struct {
    executor adapters.QueryExecutor
    cfg      AuthConfig
}

// BAD: god handler
type APIHandler struct {
    adapter adapters.Adapter // auth, CRUD, catalog, scripts...
}
```

### O — Open/Closed

Extend behavior via new types and interfaces, not by editing stable call sites.

**pREST mapping:** Add a new port in `adapters/` (e.g. `PermissionsChecker`), implement in `adapters/postgres/`, inject via `controllers.Deps`. Optional lifecycle (`DatabaseConnector`, `DatabasePinger`) stays outside composite `Adapter` so mocks stay minimal.

```go
// GOOD: extend via new port + wiring
type Deps struct {
    Perms adapters.PermissionsChecker // new concern → new field
}

// BAD: modify CRUDHandler for every new cross-cutting concern
func (h *CRUDHandler) handle(w http.ResponseWriter, r *http.Request) {
    if newFeatureFlag { /* special case */ }
}
```

### L — Liskov Substitution

Any implementation of a port must satisfy the full contract callers rely on.

**pREST mapping:** `adapters/mockgen/` and `adapters/postgres/` must be interchangeable for the ports handlers use. Test doubles must not panic or return inconsistent results on methods production code calls.

```go
// GOOD: mock implements the port handlers actually call
ctrl := gomock.NewController(t)
exec := mockgen.NewMockQueryExecutor(ctrl)
exec.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

// BAD: partial stub that panics on unimplemented methods
type brokenExecutor struct{}
func (brokenExecutor) QueryCtx(...) adapters.Scanner { panic("not implemented") }
```

### I — Interface Segregation

Depend on the smallest interface that suffices; avoid fat interfaces at call sites.

**pREST mapping:** `NewAuthHandler(executor adapters.QueryExecutor, …)` needs only query execution. `NewTableHandler(executor, db, singleDB)` needs executor + registry, not full `Adapter`. Split ports live in `adapters/*.go`; composite `Adapter` is for the composition root only.

```go
// GOOD: narrow dependency
type TableHandler struct {
    executor adapters.QueryExecutor
    db       adapters.DatabaseRegistry
}

// BAD: forces mocks to implement 50+ methods
type TableHandler struct {
    adapter adapters.Adapter
}
```

### D — Dependency Inversion

High-level modules (`controllers/`, `middlewares/`) depend on abstractions (`adapters/*`), not concretions (`adapters/postgres`).

**pREST mapping:** `controllers.NewDepsFromConfig` maps `p.Adapter` into `Deps` port fields. Concrete postgres wiring happens in `cmd/prestd/` and `router/`, not inside handlers.

```go
// GOOD: depend on port, inject at edge
func NewCRUDHandler(deps Deps) *CRUDHandler {
    return &CRUDHandler{
        executor: deps.Executor, // adapters.QueryExecutor
    }
}

// BAD: construct concrete adapter inside handler
func NewCRUDHandler(cfg *config.Prest) *CRUDHandler {
    pg := postgres.New(cfg) // inverted dependency
}
```

### Quick reference

| Principle | pREST shorthand | Primary packages |
|-----------|-----------------|------------------|
| SRP | One handler/port per concern | `controllers/*`, `adapters/*.go` |
| OCP | New port + adapter, not `switch` hacks | `adapters/`, `controllers/deps.go` |
| LSP | mockgen and postgres interchangeable | `adapters/mockgen/`, tests |
| ISP | Smallest port on handler structs | `controllers/`, `adapters/` |
| DIP | No `adapters/postgres` outside wiring | `cmd/`, `router/`, `integration/` |

## Code Organization Conventions

- Keep new code in the most relevant existing package instead of creating new top-level folders.
- Follow existing package boundaries and hexagonal roles (see **Hexagonal Architecture** below).
- Keep HTTP-related logic in controllers/middlewares and DB-specific logic in `adapters/postgres/`.
- Avoid cyclic imports; depend on narrow ports in `adapters/`, not concrete implementations.

## Go Style and Implementation Rules

- Use idiomatic Go and `gofmt` formatting.
- Prefer standard library first; add dependencies only when clearly justified.
- Reuse existing dependencies already present in `go.mod` where suitable.
- Return wrapped errors with context using `%w` when propagating errors.
- Keep functions cohesive and reasonably short.
- Avoid package-level mutable application state. pREST is stateless; pass config and dependencies via constructors, `Execute`/Cobra command context, or request scope — not globals in `cmd/`, handlers, or adapters. (The postgres test checklist applies the same rule to unit tests.)
- Add comments only when logic is non-obvious.

## API and SQL Safety

- Do not weaken authentication/authorization logic.
- Preserve current validation and sanitization behavior.
- Never concatenate untrusted input directly into SQL.
- Keep compatibility with the current PostgreSQL adapter/query conventions in `adapters/postgres/`.
- For endpoint changes, ensure router/controller contracts remain aligned.

## Testing Expectations

- Develop behavior changes with **TDD**; maintain **≥80% package coverage** for packages under change. See `.cursor/rules/unit-tests-tdd.mdc`.
- Add or update tests for behavior changes.
- **Unit tests:** co-located `*_test.go` in the source package; use gomock of narrow adapter interfaces; never call `postgres.Load()` or hit a real database.
- **Integration tests:** only under `integration/`, mirroring package layout; exercise public HTTP/adapter surfaces end-to-end with Docker Postgres and **deployed prestd processes** over the network (see **Network integration tests** below).
- Never add `postgres.Load()` outside `integration/`.
- Reuse existing test patterns (`testify`, `adapters/mockgen/`, `integration/testutils/` for HTTP helpers).
- Mock **ports** (`adapters/*` interfaces), not `adapters/postgres` types, in unit tests outside `adapters/postgres/`.
- Name test files after the source file under test: `<source_file>_test.go` (e.g. `catalog.go` → `catalog_test.go`).
- Any new change in behavior on the controllers package should introduce a new integration test as well as a unit test.
- Prefer `t.Parallel()` in unit tests when safe (no shared mutable state, globals, or ordering dependencies). Subtests in table-driven tests can call `t.Parallel()` inside `t.Run`.
- **Do not** call `t.Parallel()` when a test uses `t.Setenv`, `config.Load()` against mutated env, package-global hooks (`SetDBConnectForTest` / `withFailingDBConnect`), shared package-level config vars, or intentional concurrency tests (e.g. plugin serialization). Keep those serial.
- `make test-unit` is the canonical unit-test entry point: **30s** per-package timeout (`-timeout 30s`), tests within a package run in parallel up to `GOMAXPROCS` (`-parallel`), packages are invoked in batch (also concurrent), and the race detector is enabled (`-race`).

### Postgres adapter unit tests (`adapters/postgres`)

Unit tests under `adapters/postgres/**` must pass with **no Postgres process running**. Real DB usage belongs only in `integration/postgres/adapters/postgres/**` (Docker).

| Location | Package | Database | Run via |
|----------|---------|----------|---------|
| `adapters/postgres/**` | unit | **Never real** — sqlmock only | `go test ./adapters/postgres/...`, `make test-unit` |
| `integration/postgres/adapters/postgres/**` | integration | Real Postgres (Docker) | `make test-integration-postgres` |

### Network integration tests

`make test-integration-postgres` (`integration/postgres/docker-compose.yml`) provisions Postgres, seeds data via `testdata/db-init.sh`, starts **real prestd servers**, then runs `./integration/suites/...` and `./integration/postgres/...`.

`make test-integration-timescaledb` (`integration/timescaledb/docker-compose.yml`) provisions TimescaleDB and runs `./integration/timescaledb/...` **only** (does not re-run shared `suites/` or Postgres packages).

| Service | URL env (in tests container) | Config |
|---------|------------------------------|--------|
| `prestd` | `PREST_TEST_URL=http://prestd:3000` | `testdata/prest.toml`, debug on, JWT off |
| `prestd-multicluster` | `PREST_MULTICLUSTER_TEST_URL=http://prestd-multicluster:3001` | `testdata/prest_multicluster.toml` |
| `prestd-auth` | `PREST_AUTH_TEST_URL=http://prestd-auth:3002` | auth enabled |

Standard-stack HTTP tests use [`integration/helpers/server.go`](integration/helpers/server.go):

- `helpers.ServerURL(t)` — default prestd; `t.Skip` when env unset
- `helpers.MultiClusterServerURL(t)` — multi-cluster prestd
- `helpers.AuthServerURL(t)` — auth-enabled prestd

Call deployed servers with `integration/testutils.DoRequest(t, base+path, ...)`. Do **not** use `httptest.NewServer(helpers.IntegrationHandler(...))` for controller/router tests unless the test needs a **custom negroni stack** or per-test config mutation (e.g. `integration/middlewares/`, `integration/plugins/`, `TestSilentErrorsOnQuery`).

Keep [`helpers.IntegrationHandler`](integration/helpers/setup.go) for custom-stack tests only. Adapter-level tests under `integration/postgres/adapters/` may still call the adapter directly.

Local `go test ./integration/...` without compose skips network tests (no `PREST_*_TEST_URL`).

**Golden rule:** never call `conn.Get()`, `Connect()`, `DB()`, or `Ping()` in a way that reaches `sqlx.Connect` without a test double in place.

Allowed patterns only:

1. **Happy path / SQL behavior** — `sqlmock` + `InjectDBForTest`
2. **Connection failure path** — `SetDBConnectForTest` returning an error (no TCP)
3. **Pure logic** — no adapter DB calls (request parsing, SQL builders, permissions, etc.)

**Standard helpers (`package postgres`)**

| Helper | Use when | What it does |
|--------|----------|--------------|
| `defaultTestConf()` | Building config for adapter tests | Returns minimal `*config.Prest`; does **not** imply a live DB |
| `withSQLMock(t)` | Single-database SQL tests | `sqlmock.New` → `InjectDBForTest` for `defaultMockDB`; registers `t.Cleanup` for pool + stmt cache |
| `withSQLMocks(t)` | Context DB switching (`pctx.DBNameKey`) | Injects `defaultMockDB` + `contextMockDB` pools |
| `withSQLMockPing(t)` | `Connect` / `Ping` success paths | Like `withSQLMock` + `MonitorPingsOption` |
| `withFailingDBConnect(t, msg)` | Connection error paths | Stubs `dbConnect` to return error immediately; no network |

**Connection manager test API (`internal/connection`)**

| API | Purpose |
|-----|---------|
| `InjectDBForTest(uri, db)` | Register a `*sqlx.DB` (sqlmock-backed) in the pool |
| `ResetPoolForTest()` | Clear pool between tests (`t.Cleanup`) |
| `SetDBConnectForTest(fn)` | Stub `sqlx.Connect` for connection-failure tests from parent package |

Within `package connection` tests, `dbConnect` may be assigned directly (same package).

**Per-test setup checklist** (when a test touches the database layer):

- Use `withSQLMock` / `withSQLMocks` / `withFailingDBConnect` — not bare `New(cfg)` for DB paths
- Register `t.Cleanup` for pool reset and stmt cache (helpers do this)
- Do **not** set `PGConnTimeout` hoping TCP fails fast — stub instead
- Do **not** add package-level test globals; per-test `New(cfg)` + cleanup only

**Test file layout**

| File | Contents |
|------|----------|
| `postgres_test.go` | Pure logic, SQL builders, permissions, table-driven `delete`/`update`/`BatchInsertCopy` |
| `postgres_exec_test.go` | `withSQLMock*` helpers + executor integration with sqlmock |
| `postgres_conn_test.go` | `Connect`, `Ping`, `DB`, stmt cache |
| `queries_test.go` | Scripts / `WriteSQL` via sqlmock |
| `internal/connection/conn_test.go` | Pool, singleflight; stubs `dbConnect` inline |
| `formatters/formatters_test.go` | No DB |

**Timeout safety net** (guards against regressions, not substitutes for mocking):

- **30s** per-package via `make test-unit` (`go test -timeout 30s -parallel $GOMAXPROCS …` for all unit packages)

**Anti-patterns (do not do)**

```go
// BAD: reaches real sqlx.Connect → TCP hang
adapter := New(defaultTestConf()).(*postgres)
adapter.BatchInsertCopy(...)

// BAD: relies on network / PGConnTimeout
cfg.PGConnTimeout = 1
adapter.Connect()

// GOOD: connection error without network
adapter := withFailingDBConnect(t, "connect failed")

// GOOD: SQL path with sqlmock
adapter, mock := withSQLMock(t)
mock.ExpectPrepare(...)
```

Common commands:

```sh
# Unit tests (no database, parallel, 30s timeout per package, race detector)
make test          # or: make test-unit
# Equivalent to two Makefile steps:
#   go test -timeout 30s -parallel $GOMAXPROCS -race … ./adapters/postgres/...
#   go test -timeout 30s -parallel $GOMAXPROCS -race … $(other unit packages)

# Postgres adapter unit tests only (sqlmock, 30s timeout, parallel)
go test -timeout 30s -parallel 8 ./adapters/postgres/...

# Integration tests (Docker Postgres, integration/ tree only)
make test-integration
go test ./integration/...
```

## Config and CLI Changes

- Keep config behavior consistent with existing `prest.toml` and environment variable conventions.
- For CLI updates, follow existing Cobra command patterns in `cmd/`.
- Avoid renaming/removing existing flags or commands unless explicitly requested.

### Config resilience policy

Wrong or partial configuration must **not** block API startup. Log `slog.Warn`, fall back to viper defaults or safe zero values, and continue. Reference implementations live in `config/config.go`: `ensureCacheStorage`, `ensureQueriesPath`, `ensureJWTConfig`, `unmarshalKeyOrZero`, and `getJSONAgg`.

**Patterns when adding config keys**

| Kind | On invalid/missing | Helper |
|------|-------------------|--------|
| Scalar | Use `viper.SetDefault` + `Get*` | viper defaults |
| Slice/struct TOML key | Warn + zero value | `unmarshalKeyOrZero` |
| Optional filesystem path | Configured → default → disable feature | `ensure*Path` (cache, queries) |
| Auth/JWT misconfiguration | Warn + disable feature | `ensureJWTConfig` |
| Enum-like value | Warn + default | `getJSONAgg` pattern |

**Defense-in-depth at request time** (unchanged):

- Middleware empty-key guards in `middlewares/` — refuse tokens when verification material is missing (GHSA-fj7v-859r-2fm4).
- SQL/auth validation in middleware — unchanged.

**Parse / Load behavior**

- Missing, unreadable, or malformed TOML: warn and use viper defaults + `PREST_*` env overrides.
- Invalid `access.tables`, `access.users`, `pluginmiddlewarelist`, `cache.endpoints`: warn and use empty slices.
- Queries or cache storage path unavailable: warn, retry default path, disable feature if both fail.
- Auth enabled without `jwt.key`, or `jwt.default` enabled without verification material: warn and disable the feature (`jwt.default` defaults to `false`).

## Performance and Reliability

- Avoid unnecessary allocations and repeated DB work in hot paths.
- Ensure resources are closed (`rows.Close`, response bodies, etc.).
- Preserve thread-safety assumptions in cache, middleware, and shared components.

## Documentation and Developer Experience

- Update docs/comments only when behavior changes or new setup is required.
- Keep examples and guidance consistent with current README/Makefile workflows.
- Do not add unrelated formatting-only churn.

## Pull Request Quality Bar

Before finalizing changes, verify:

- Code compiles with Go compatible constructs on the project's Go version.
- Relevant tests pass for modified packages.
- New logic has tests or a clear reason why tests were not added.
- No accidental breaking change in API, CLI, or config surface.
- New types respect SOLID (narrow ports, single-purpose handlers, no concrete adapter imports in controllers).

## Hexagonal Architecture

pREST v2 uses **ports and adapters**. Business-facing HTTP code sits at the edge; PostgreSQL and other infrastructure sit behind interfaces. Dependencies point **inward** — application code never imports `adapters/postgres`.

### Layer map

| Role | Packages | Responsibility |
|------|----------|----------------|
| **Driving adapters** (primary) | `controllers/`, `middlewares/`, `router/`, `cmd/` | HTTP transport, auth, routing, CLI wiring |
| **Application core** | Handler structs + `controllers.Deps` | Orchestrate requests; depend only on ports |
| **Ports** | `adapters/` (interface files) | Contracts the core needs from the outside world |
| **Driven adapters** (secondary) | `adapters/postgres/`, `cache/`, `plugins/` | Concrete PostgreSQL, cache, and plugin implementations |
| **Composition root** | `router/`, `cmd/prestd/`, `NewDepsFromConfig` | Wire config → adapter → handlers → routes |

### Ports (`adapters/`)

Define narrow interfaces in `adapters/` — one file per concern:

- `QueryExecutor`, `RequestQueryBuilder`, `CatalogQuerier`, `SQLBuilder`
- `PermissionsChecker`, `ScriptRunner`, `DatabaseRegistry`, `Scanner`
- `Adapter` — composite of all ports; implemented only by `adapters/postgres`

Handlers should depend on the **smallest port** that suffices (e.g. `CRUDHandler` takes `QueryExecutor`, not `*postgres.Postgres`). Bundle ports in `controllers.Deps` and inject via `NewHandlers(deps)`.

### Driven adapters (`adapters/postgres/`)

- All SQL, connection pooling, `pq` usage, and Postgres-specific types live here.
- Must implement ports in `adapters/`; must not import `controllers/` or `middlewares/`.
- Shared query/scan helpers stay in this package or its subpackages (`formatters/`, `statements/`).

### Driving adapters (`controllers/`, `middlewares/`)

- Translate HTTP (path vars, headers, status codes, JSON) into port calls.
- No raw SQL, no `database/sql` or `pq` imports.
- Cross-cutting concerns (JWT, ACL, cache, exposure) belong in `middlewares/`, not controllers.
- Local handler-specific ports (e.g. `controllers.ResponseCacher`) stay in the consumer package when they are not shared infrastructure.

### Composition and wiring

- **`controllers.NewDepsFromConfig`** — maps `config.Prest` + `p.Adapter` into `controllers.Deps`.
- **`controllers.NewHandlers(deps)`** — constructs handlers from injected ports (preferred in tests).
- **`router.RegisterRoutes`** — attaches handlers and middleware stacks; no business logic.
- **`config.Prest.Adapter`** — set once at startup (`postgres.Load` / integration setup); not read directly by handlers.

### Adding a new feature (checklist)

1. **Failing unit test (TDD)** — write/adjust a failing unit test first (see `.cursor/rules/unit-tests-tdd.mdc`).
2. **Port** — add or extend an interface in `adapters/` if the core needs new external capability.
3. **Adapter** — implement it in `adapters/postgres/` (or another driven adapter package).
4. **Handler** — add methods on a handler struct; accept the port via constructor/`Deps`.
5. **Route** — register in `router/router.go`; add middleware in `middlewares/` if needed.
6. **Mocks** — run `make mockgen` for new/changed interfaces; unit-test handlers with `adapters/mockgen/`.
7. **Integration** — add `integration/<package>/` tests for end-to-end behavior with Docker Postgres.

### Dependency rules (do / don't)

**Do**

- Inject dependencies through constructors and `controllers.Deps`.
- See **SOLID Principles for Go** (especially ISP and DIP).
- Use `context.Context` on port methods that perform I/O (`*Ctx` variants on `QueryExecutor`).
- Put integration tests under `integration/` mirroring the production package layout.

**Don't**

- Import `adapters/postgres` from `controllers/`, `middlewares/`, or `router/`.
- Call `postgres.Load()` or open DB connections outside `integration/` and startup (`cmd/`).
- Add business logic to `router/` or `config/` beyond wiring, validation, and graceful degradation.
- Make handlers depend on the composite `Adapter` when a narrower port is enough.
- Leak `http.Request` or `mux.Vars` into `adapters/postgres`.

### Mock generation

Interfaces in `adapters/` are mocked via `make mockgen`, outputting to `adapters/mockgen/`. Regenerate mocks whenever a port signature changes.

## Architecture and Design Guidance

- Prefer extending existing ports and handlers over new top-level abstractions.
- Keep runtime stateless: config and dependencies flow through constructors and request/command scope, not package globals.
- When refactoring legacy code, move Postgres coupling behind an existing port rather than introducing parallel access paths.
- Plugins (`plugins/`) are optional driving/driven adapters loaded at runtime; keep their surface compatible with existing middleware and route conventions.

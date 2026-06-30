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
- Avoid global mutable states.
- Add comments only when logic is non-obvious.

## API and SQL Safety

- Do not weaken authentication/authorization logic.
- Preserve current validation and sanitization behavior.
- Never concatenate untrusted input directly into SQL.
- Keep compatibility with the current PostgreSQL adapter/query conventions in `adapters/postgres/`.
- For endpoint changes, ensure router/controller contracts remain aligned.

## Testing Expectations

- Add or update tests for behavior changes.
- **Unit tests:** co-located `*_test.go` in the source package; use gomock of narrow adapter interfaces; never call `postgres.Load()` or hit a real database.
- **Integration tests:** only under `integration/`, mirroring package layout; exercise public HTTP/adapter surfaces end-to-end with Docker Postgres.
- Never add `postgres.Load()` outside `integration/`.
- Reuse existing test patterns (`testify`, `adapters/mockgen/`, `handlerstest.NewTestHandlers`, `testutils/` for HTTP helpers).
- Mock **ports** (`adapters/*` interfaces), not `adapters/postgres` types, in unit tests.
- Maintain ≥80% coverage on new code paths (unit + integration combined).
- Name test files after the source file under test: `<source_file>_test.go` (e.g. `catalog.go` → `catalog_test.go`).

Common commands:

```sh
# Unit tests (no database, co-located in each package)
make test          # or: make test-unit
go test $(go list ./... | grep -v /integration)

# Integration tests (Docker Postgres, integration/ tree only)
make test-integration
go test ./integration/...
```

## Config and CLI Changes

- Keep config behavior consistent with existing `prest.toml` and environment variable conventions.
- For CLI updates, follow existing Cobra command patterns in `cmd/`.
- Avoid renaming/removing existing flags or commands unless explicitly requested.

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

1. **Port** — add or extend an interface in `adapters/` if the core needs new external capability.
2. **Adapter** — implement it in `adapters/postgres/` (or another driven adapter package).
3. **Handler** — add methods on a handler struct; accept the port via constructor/`Deps`.
4. **Route** — register in `router/router.go`; add middleware in `middlewares/` if needed.
5. **Mocks** — run `make mockgen` for new/changed interfaces; unit-test handlers with `adapters/mockgen/`.
6. **Integration** — add `integration/<package>/` tests for end-to-end behavior with Docker Postgres.

### Dependency rules (do / don't)

**Do**

- Inject dependencies through constructors and `controllers.Deps`.
- Keep ports small and purpose-specific (interface segregation).
- Use `context.Context` on port methods that perform I/O (`*Ctx` variants on `QueryExecutor`).
- Put integration tests under `integration/` mirroring the production package layout.

**Don't**

- Import `adapters/postgres` from `controllers/`, `middlewares/`, or `router/`.
- Call `postgres.Load()` or open DB connections outside `integration/` and startup (`cmd/`).
- Add business logic to `router/` or `config/` beyond wiring and validation.
- Make handlers depend on the composite `Adapter` when a narrower port is enough.
- Leak `http.Request` or `mux.Vars` into `adapters/postgres`.

### Mock generation

Interfaces in `adapters/` are mocked via `make mockgen`, outputting to `adapters/mockgen/`. Regenerate mocks whenever a port signature changes.

## Architecture and Design Guidance

- Prefer extending existing ports and handlers over new top-level abstractions.
- When refactoring legacy code, move Postgres coupling behind an existing port rather than introducing parallel access paths.
- Plugins (`plugins/`) are optional driving/driven adapters loaded at runtime; keep their surface compatible with existing middleware and route conventions.

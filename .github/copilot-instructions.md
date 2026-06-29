# Copilot Instructions for pREST Go Backend

These instructions guide AI-generated changes in this repository.

## Project Scope

- Repository: `github.com/prest/prest`
- Language: Go (`go 1.25.0`, always check `go.mod`)
- Primary domain: PostgreSQL-backed REST API server, CLI, middleware, adapters, and plugins.
- Runtime entrypoint: `cmd/prestd/main.go`

## Core Engineering Principles

- Prefer small, focused changes over broad refactors.
- Preserve existing public behavior unless the task explicitly requires a change.
- Keep compatibility with Go 1.25.0.
- Favor readability and maintainability over clever abstractions.
- Do not introduce breaking changes to command flags, config keys, routes, or plugin behavior without explicit request.

## Code Organization Conventions

- Keep new code in the most relevant existing package instead of creating new top-level folders.
- Follow existing package boundaries (examples: `controllers/`, `middlewares/`, `adapters/`, `plugins/`, `router/`, `config/`).
- Keep HTTP-related logic in controllers/middlewares and DB-specific logic in adapter/postgres layers.
- Avoid cyclic imports; prefer dependency injection and interfaces where patterns already exist.

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
- Prefer package-local unit tests near changed code.
- Reuse existing test patterns (`*_test.go`, `testify`, existing mocks in `adapters/mock/`, `testutils/`).
- When integration behavior is affected, use existing Docker-based test workflow.
- All added code must be covered by tests unless explicitly justified.
- Added tests must cover at least 80% of new code paths and edge cases.

Common commands:

```sh
# Unit/integration flow used by repository
make test

# Local package-level tests
go test ./...
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

## Architecture and Design Guidance

- Current architecture is based on a layered design: controllers, middlewares, adapters, and plugins.
- Current v2 architecture is designed to follow a hexagonal architecture pattern, with http handlers/controllers on the outside and adapters for database access on the inside.

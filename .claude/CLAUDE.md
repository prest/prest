# Claude Guidelines for pREST

This file documents how Claude should work on the pREST project.

## Overview

pREST is a PostgreSQL REST API framework written in Go. This document specifies how Claude should approach development, testing, and architecture decisions.

**Primary source of truth:** `.cursor/rules/` — All rules are defined in Cursor rules. Claude follows them directly.

## Key Principles

### 1. Architecture: Hexagonal with Dependency Injection

All changes must follow hexagonal architecture principles:
- **Ports** (interfaces) defined in `adapters/` package
- **Adapters** (implementations) in `adapters/<engine>/` (e.g., `adapters/postgres/`, `adapters/timescaledb/`)
- **Handlers** depend on ports, not concrete adapters
- **Composition root** in `app/app.go` wires everything together via dependency injection

**Reference:** `.cursor/rules/hexagonal-architecture.mdc`

### 2. Database Adapter Pattern

pREST supports **multiple SQL database engines simultaneously** via an adapter registry.

**Architecture:**
- At startup, each configured database gets assigned an adapter (Postgres, TimescaleDB, MySQL, etc.)
- At request time, the router determines which database is being queried and selects the corresponding adapter
- Handlers receive adapters via `controllers.Deps`, not via global state
- The adapter registry (`adapters.Registry`) maps database aliases to adapters

**Adding a new database:**
1. Classify it (wire-compatible variant vs. new dialect) — see `SQL_DATABASE_SUPPORT.md`
2. Create gap analysis (`integration/<db>/DIFFERENCES.md`)
3. Implement ports/adapters in `adapters/<engine>/`
4. Register in composition root
5. Add integration tests in isolated `integration/<db>/` job

**Reference:** `.cursor/rules/sql-database-support.mdc`

### 3. Testing: TDD + Isolation

**Unit tests (TDD):**
- Write failing test first for all behavior changes
- Mock ports, never live databases
- Achieve ≥80% coverage on touched packages
- Use `adapters/mockgen/` for test doubles

**Integration tests:**
- Per-database isolation: `integration/<db>/` runs only that DB's tests
- Shared wire-compatible tests: `integration/suites/` runs on Postgres job only
- Each test documented with 1–3 comment lines explaining scenario
- Use `helpers.ServerURL` and `testutils.DoRequest`, not `httptest.NewServer`

**Reference:** `.cursor/rules/unit-tests-tdd.mdc`, `.cursor/rules/integration-tests.mdc`

### 4. Response Discipline

Claude outputs should be:
- Concise: code-first, minimal prose
- Patch-based: show only changed lines + 3–5 lines context
- No summaries: user can read the diff
- No affirmations ("Sure!", "Great question")

**Reference:** `.claude/rules/CORE.md`

### 5. Git Workflow

Claude **prepares** commits, humans **execute** them.

**Process:**
1. Run `git status`, `git diff`, `git log -10` (read-only)
2. Draft commit message focused on **why**, not **what**
3. Provide copy-paste commands for human to run
4. Never run `git commit`, `git add` (staging), `git push --force`, or skip hooks

**Commit style:** Use Conventional Commits (`feat:`, `fix:`, `refactor:`, `chore:`)

**Reference:** `.claude/rules/GIT_WORKFLOW.md`

## When to Apply Rules

See `.cursor/rules/` for all authoritative rules:

| Situation | Rule File |
|-----------|-----------|
| Every request | `core.mdc` (response discipline) |
| Code changes | `hexagonal-architecture.mdc` (DIP/ISP) |
| Behavior changes | `unit-tests-tdd.mdc` (TDD, ≥80% coverage) |
| New database engine | `sql-database-support.mdc` (classify, DIFFERENCES.md, adapter registry) |
| Adapter tests | `adapter-unit-tests.mdc` (no live DB, sqlmock only) |
| Integration tests | `integration-tests.mdc` (readability, helpers, placement) |
| Test infrastructure | `integration-layout.mdc` (per-DB isolation) |
| Commits/PRs | `git-commits.mdc` (prepare, don't execute) |
| Token efficiency | `token-optimization.mdc` (parallel calls, caching) |

## Scope & Patterns

### Multi-Adapter System

pREST allows a single API server to serve **multiple SQL databases**, each with its own adapter:

```text
Request: GET /mydb/public/users
  ↓
Router looks up "mydb" in adapter registry
  ↓
Router gets adapter for "mydb" (e.g., postgres, timescaledb, mysql)
  ↓
Handler receives correct adapter via Deps
  ↓
Adapter executes query on correct database
```

The adapter registry is the source of truth for which adapter handles which database.

### Request Routing

Requests must include the database name in the path (`/{dbname}/`). The router extracts this and:
1. Looks up the database in the adapter registry
2. Retrieves the corresponding adapter
3. Passes the adapter to handlers via dependency injection
4. Handlers are database-agnostic (depend on ports, not implementations)

### Composition Root

`app/app.go` is where adapters are:
- Created (one per configured database)
- Connected (pooled connections established)
- Registered (added to registry)
- Injected (passed to handlers via `controllers.Deps`)

**Do not:**
- Create adapters in handlers or middleware
- Import concrete adapter types outside composition root
- Use globals or singletons for adapter state

## Feature Development Checklist

When adding a feature that touches adapters:

- [ ] Failing unit test (TDD) with mock adapter
- [ ] Port interface (if new capability)
- [ ] Adapter implementation
- [ ] Handler/middleware integration via `Deps`
- [ ] Route registration
- [ ] Integration test (real DB)
- [ ] All tests pass (unit + integration)
- [ ] No handler/middleware imports concrete adapter
- [ ] Commit message focused on why

When adding a new database engine:

- [ ] Classify (wire-compatible or new dialect)
- [ ] Write DIFFERENCES.md
- [ ] Create `adapters/<engine>/` implementing ports
- [ ] Add to adapter registry
- [ ] Create `integration/<engine>/` tests
- [ ] Add `.github/workflows/test-integration-<engine>.yml`
- [ ] Update `Makefile` with `test-integration-<engine>` target
- [ ] All existing tests still pass

## Related Documentation

- **Architecture samples:** `.github/copilot-instructions.md`
- **SQL DB support playbook:** skill `sql-database-support`
- **Integration test templates:** skill `prest-integration-tests`
- **Database comparison:** `integration/*/DIFFERENCES.md`

## Questions?

Refer to:
1. `.cursor/rules/` for all authoritative rules
2. `CLAUDE.md` (this file) for multi-adapter architecture and composition patterns
3. Existing code in `adapters/postgres/` and `app/app.go` for implementation examples

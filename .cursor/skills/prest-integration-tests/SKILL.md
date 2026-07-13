---
name: prest-integration-tests
description: >-
  Guides writing and reviewing pREST Docker/network integration tests under
  integration/ so each request is human-readable via step comments or
  table-driven descriptions. Use when adding or editing
  integration/**/*_test.go, HTTP controller E2E coverage, make
  test-integration, or when the user asks for integration tests.
---

# pREST Integration Tests

Integration tests must be **human-readable**. A reader should understand the
scenario, expected outcome, and why each request matters without decoding URLs
or status codes alone.

Gold standard: `integration/controllers/queries_database_test.go`.
Side-by-side patterns: [examples.md](examples.md).

## When writing or editing

1. Place tests only under `integration/`, mirroring production packages
   (`controllers/`, `middlewares/`, `adapters/`, …).
2. Document every request (see below).
3. Prefer deployed-server helpers over in-process HTTP servers.
4. Validate with `make test-integration` (local `go test ./integration/...`
   skips when `PREST_*_TEST_URL` is unset).

## Human-readable step docs (required)

### Sequential / imperative tests

Before **every** HTTP request, add 1–3 comments covering:

1. **What** — endpoint or scenario under test
2. **Expected outcome** — succeed or fail, and what that means
3. **Why** (when non-obvious) — path params, auth, DB selection, etc.

Template:

```go
// Test the <endpoint or scenario>
// Expected to succeed|fail and <outcome summary>.
// <optional: why / which path param or auth rule matters>
helpers.DoAuthRequest(...)
```

Also:

- Wrap long `DoRequest` / `DoAuthRequest` calls across lines for scanability.
- Use a stable scenario name string (last arg) that matches the comment intent
  (e.g. `"QueriesDBExecuteWithDB"`).

### Table-driven tests

Skip per-request block comments when each case has a clear `description`
field. Descriptions must state scenario + expected outcome in plain language.

- Good: `"Get tables with custom where invalid clause"`
- Bad: `"case 1"`, `"ok"`, `"err"`

Log or surface the description in the loop (`t.Log(tc.description)` or
`t.Run(tc.description, …)`).

## Structural placement

| Do | Don't |
|----|-------|
| `helpers.ServerURL`, `QueriesServerURL`, `AuthServerURL`, `MultiClusterServerURL` | `httptest.NewServer` for standard controller routes |
| `testutils.DoRequest` / `helpers.DoAuthRequest` | Call `postgres.Load()` or live DB outside `integration/` |
| Custom Negroni/config only when needed (`middlewares/`, `plugins/`) | Invent new HTTP client helpers when existing ones suffice |

Controller behavior changes need unit tests **and** integration coverage under
`integration/`.

## Checklist

Before finishing a new or edited integration test:

- [ ] File lives under `integration/`
- [ ] Every request documented (step comments) **or** table `description` is self-explanatory
- [ ] Expected status and failure cases called out in the docs
- [ ] Uses existing helpers; no live DB via `postgres.Load()` outside `integration/`
- [ ] Controller behavior changes include both unit + this integration coverage

# Mock adapter for tests

The `adapters/mock` package provides a lightweight in-memory adapter used by
unit tests when a real PostgreSQL connection is unnecessary.

## Why use it

- Fast tests without Docker/PostgreSQL startup.
- Deterministic responses for edge-case scenarios.
- Clear separation between behavior tests and integration tests.

## Basic setup

```go
package mypkg_test

import (
	"testing"

	mockadapter "github.com/prest/prest/adapters/mock"
)

func TestWithMockAdapter(t *testing.T) {
	adapter := mockadapter.New()
	if adapter == nil {
		t.Fatal("expected mock adapter")
	}
}
```

## Suggested strategy

- Use `adapters/mock` in pure unit tests.
- Use `docker-compose-test.yml` integration flow when validating SQL behavior.

This split keeps the feedback loop short while preserving confidence in
database-related features.

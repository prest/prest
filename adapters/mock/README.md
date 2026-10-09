# Mock Adapter

The `adapters/mock` package provides a test-only implementation of the pREST
`adapters.Adapter` interface. It lets tests exercise code that depends on an
adapter without opening a PostgreSQL connection.

Use it when a test needs to control adapter responses directly. It is not an
in-memory database, and it does not parse or execute SQL. Methods such as
`Query`, `Insert`, `Update`, `Delete`, `QueryCount`, and batch insert helpers
return the next queued item as a `scanner.PrestScanner`.

## Basic usage

Create a mock adapter with the current test handle, enqueue each expected
adapter response with `AddItem`, then call the method under test.

```go
package example_test

import (
	"testing"

	"github.com/prest/prest/v2/adapters/mock"
)

func TestQueryUsers(t *testing.T) {
	adapter := mock.New(t)
	adapter.AddItem([]byte(`[{"id":1,"name":"Ada"}]`), nil, false)

	scanner := adapter.Query(`select id, name from users`)
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	var users []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	count, err := scanner.Scan(&users)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
}
```

## Queueing responses

`AddItem` appends responses to a FIFO queue:

```go
adapter.AddItem([]byte(`[{"id":1}]`), nil, false)
adapter.AddItem(nil, errors.New("database unavailable"), false)
adapter.AddItem(nil, nil, true)
```

Each call to a supported adapter method consumes one queued item. If a method
that needs a queued item is called with an empty queue, the mock fails the test
through `testing.T`.

The `isCount` argument is used by `DatabaseClause` and `SchemaClause` to return
the `hasCount` value expected by callers that branch on count requests.

## Transactions

`New(t)` registers a `database/sql` driver named `mock` if it has not already
been registered. `GetTransaction` and `GetTransactionCtx` open that driver with
the `prest` DSN and return a transaction whose `Commit` and `Rollback` methods
are no-ops.

## Scope

The mock adapter is intentionally small:

- It verifies at compile time that `Mock` implements `adapters.Adapter`.
- It mirrors table and field permission behavior enough for adapter-dependent
  tests.
- Most SQL construction helpers return zero values because SQL generation is
  covered by the real PostgreSQL adapter tests.

Run the package tests with:

```bash
go test ./adapters/mock
```

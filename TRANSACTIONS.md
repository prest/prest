# Transactions

pREST supports distributed HTTP-level database transactions using the **Saga pattern**. Transaction metadata is stored in the main database, while operations are executed on the target database during commit. This enables multi-database and multi-instance deployments.

## Overview

The transaction workflow follows a simple pattern:

1. **Start** a transaction to get a transaction ID
2. **Execute** CRUD operations using the transaction ID in the `Authorization-Transaction` header (operations are staged, not executed immediately)
3. **Commit** to execute all staged operations atomically on the target database, or **Rollback** to discard them

Transactions are stored in PostgreSQL tables (`prest_transactions` and `prest_transaction_operations`) in the main database, making them accessible from any pREST instance.

## API Reference

### Start a Transaction

```
POST /{database}/{schema}/transactions
```

Creates a new transaction and returns a transaction ID.

**Response:**
```json
{
  "tx": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

**Status:** `201 Created`

### List Open Transactions

```
GET /{database}/{schema}/transactions
```

Returns all pending transactions for the specified database and schema.

**Response:**
```json
[
  {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "database": "mydb",
    "schema": "public",
    "status": "pending",
    "operation_count": 3,
    "created_at": "2026-07-26T10:30:00Z"
  }
]
```

**Status:** `200 OK`

### Get Transaction Status

```
GET /{database}/{schema}/transactions/{txID}
```

Returns the status and metadata of a specific transaction.

**Response:**
```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "database": "mydb",
  "schema": "public",
  "status": "pending",
  "operation_count": 3,
  "created_at": "2026-07-26T10:30:00Z"
}
```

**Status:** `200 OK`

### Commit Transaction

```
POST /{database}/{schema}/transactions/{txID}/commit
```

Executes all staged operations atomically on the target database. The target database is resolved using the transaction's database name via the adapter registry.

**Response:**
```json
{
  "status": "committed"
}
```

**Status:** `200 OK`

### Rollback Transaction

```
POST /{database}/{schema}/transactions/{txID}/rollback
```

Discards all staged operations. No changes are made to any database.

**Response:**
```json
{
  "status": "rolled_back"
}
```

**Status:** `200 OK`

## Using Transactions

### Basic Flow

```bash
# 1. Start a transaction
TX_ID=$(curl -s -X POST http://localhost:3000/mydb/public/transactions \
  -H "Authorization: Bearer <token>" | jq -r '.tx')

# 2. Insert a record (operation is staged, not executed)
curl -X POST http://localhost:3000/mydb/public/orders \
  -H "Authorization: Bearer <token>" \
  -H "Authorization-Transaction: $TX_ID" \
  -H "Content-Type: application/json" \
  -d '{"product_id": 42, "quantity": 10, "status": "pending"}'
# Response: 202 Accepted {"status":"pending","tx":"a1b2c3d4..."}

# 3. Update another record (also staged)
curl -X PUT http://localhost:3000/mydb/public/inventory?product_id=42 \
  -H "Authorization: Bearer <token>" \
  -H "Authorization-Transaction: $TX_ID" \
  -H "Content-Type: application/json" \
  -d '{"stock": "stock - 10"}'
# Response: 202 Accepted {"status":"pending","tx":"a1b2c3d4..."}

# 4. Check transaction status (shows operation count)
curl http://localhost:3000/mydb/public/transactions/$TX_ID \
  -H "Authorization: Bearer <token>"
# Response: {"id":"...","status":"pending","operation_count":2,...}

# 5. Commit all operations atomically on "mydb"
curl -X POST "http://localhost:3000/mydb/public/transactions/$TX_ID/commit" \
  -H "Authorization: Bearer <token>"
# Response: 200 OK {"status":"committed"}
```

### Rollback Example

```bash
# Start transaction
TX_ID=$(curl -s -X POST http://localhost:3000/mydb/public/transactions \
  -H "Authorization: Bearer <token>" | jq -r '.tx')

# Perform operations (staged)
curl -X POST http://localhost:3000/mydb/public/orders \
  -H "Authorization: Bearer <token>" \
  -H "Authorization-Transaction: $TX_ID" \
  -H "Content-Type: application/json" \
  -d '{"product_id": 42, "quantity": 10}'

# Rollback - operations are discarded, nothing persists
curl -X POST "http://localhost:3000/mydb/public/transactions/$TX_ID/rollback" \
  -H "Authorization: Bearer <token>"
```

### Supported Operations

Within a transaction, you can use the following CRUD operations with the `Authorization-Transaction` header:

- **INSERT:** `POST /{database}/{schema}/{table}`
- **UPDATE:** `PUT/PATCH /{database}/{schema}/{table}`
- **DELETE:** `DELETE /{database}/{schema}/{table}`

**Note:** `SELECT` operations are not affected by transactions and always read committed data. Staged operations return `202 Accepted` instead of the normal response.

## Architecture

### Saga Pattern

Instead of holding a real PostgreSQL transaction open (which is tied to a single database connection), pREST uses the **Saga pattern**:

1. **Start:** Create a transaction record in `prest_transactions` (main database)
2. **Execute:** Each CRUD operation is recorded in `prest_transaction_operations` with the SQL query and parameters (main database)
3. **Commit:** 
   - Lock the transaction in the main database
   - Resolve the target database via the adapter registry
   - Open a real database transaction on the **target database**
   - Execute all staged operations in order
   - Commit the target database transaction
   - Update status and clean up in the main database
4. **Rollback:** Simply delete the staged operations from the main database (no database changes)

### Database Separation

```
┌─────────────────────────────────────────────────────────────────┐
│                     Main Database                                │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ prest_transactions                                          │ │
│  │ prest_transaction_operations                                │ │
│  │ (transaction metadata + staged operations)                  │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Commit: resolve target DB
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Target Database (mydb)                         │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ orders                                                      │ │
│  │ inventory                                                   │ │
│  │ users                                                       │ │
│  │ (actual business data)                                       │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
1. POST /mydb/public/transactions
   → TransactionHandler.Start()
     → INSERT INTO prest_transactions (main DB)
     → 201 {"tx": "a1b2c3d4"}

2. POST /mydb/public/orders (Authorization-Transaction: a1b2c3d4)
   → TransactionMiddleware validates txID exists and is pending
   → CRUDHandler.Insert()
     → INSERT INTO prest_transaction_operations (main DB)
     → 202 {"status": "pending", "tx": "a1b2c3d4"}

3. POST /mydb/public/transactions/a1b2c3d4/commit
   → TransactionHandler.Commit()
     → BEGIN on main DB (lock transaction)
     → Read operations from prest_transaction_operations
     → Resolve "mydb" adapter from registry
     → BEGIN on target DB (mydb)
     → Execute all staged operations
     → COMMIT on target DB
     → UPDATE prest_transactions SET status = 'committed'
     → DELETE FROM prest_transaction_operations
     → COMMIT on main DB
     → 200 {"status": "committed"}
```

### Database Tables

```sql
-- Transaction metadata (in main database)
CREATE TABLE prest_transactions (
    id VARCHAR(36) PRIMARY KEY,
    database_name VARCHAR(255) NOT NULL,
    schema_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Staged operations (in main database)
CREATE TABLE prest_transaction_operations (
    id SERIAL PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES prest_transactions(id) ON DELETE CASCADE,
    operation VARCHAR(10) NOT NULL,
    table_name TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    params JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

### Cleanup

A background goroutine runs every 60 seconds and deletes expired transactions (default TTL: 30 minutes). Expired transactions and their staged operations are automatically removed from the main database.

## Configuration

### Transaction TTL

The TTL is configured in `app/app.go` when creating the `TransactionManager`:

```go
txManager := transactions.NewManager(db, 30 * time.Minute)
```

### Connection Pool

- **Main database:** Used for transaction metadata and staged operations
- **Target databases:** Resolved via the adapter registry during commit

## Multi-Database Deployment

With this architecture, you can serve multiple databases from a single pREST instance:

```
                    ┌─────────────────┐
                    │     pREST       │
                    └────────┬────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
    ┌───────▼──────┐ ┌───────▼──────┐ ┌───────▼──────┐
    │  Main DB     │ │  mydb        │ │  analytics   │
    │ (metadata)   │ │ (business)   │ │ (reporting)  │
    └──────────────┘ └──────────────┘ └──────────────┘
```

- Transaction metadata always lives in the main database
- Operations are executed on the database specified in the transaction
- Each database must be configured in `prest.toml` and registered

## Limitations

1. **Deferred execution:** Operations are not executed until commit. SELECT queries will not see pending changes.

2. **Order-dependent execution:** Operations are executed in the order they were added. If a later operation fails, earlier operations are already committed (no automatic rollback of individual operations).

3. **No isolation:** Concurrent transactions can interfere with each other. There is no pessimistic or optimistic locking.

4. **Connection pool pressure:** Commit executes all operations in a single transaction, which holds a connection for the duration.

5. **No nested transactions:** Attempting to start a new transaction within an existing one is not supported.

6. **Main database required:** Transaction metadata is always stored in the main database. The main database must be accessible and have the transaction tables created.

## Error Handling

| Scenario | HTTP Status | Error Message |
|----------|-------------|---------------|
| Transaction not found | `404 Not Found` | `transaction not found: <txID>` |
| Transaction not pending | `409 Conflict` | `transaction not found or not pending` |
| Database not found | `409 Conflict` | `database "xxx" not found in adapter registry` |
| Commit fails | `409 Conflict` | `failed to commit on target database: <details>` |
| Rollback fails | `409 Conflict` | `failed to rollback transaction: <details>` |
| Invalid path segments | `400 Bad Request` | `invalid identifier in path` |

package transactions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/prest/prest/v2/pkg/adapters"
	"github.com/prest/prest/v2/internal/adapters/postgres"
)

type TxStatus int

const (
	TxStatusPending TxStatus = iota
	TxStatusCommitted
	TxStatusRolledBack
)

func (s TxStatus) String() string {
	switch s {
	case TxStatusPending:
		return "pending"
	case TxStatusCommitted:
		return "committed"
	case TxStatusRolledBack:
		return "rolled_back"
	default:
		return "unknown"
	}
}

type ManagedTx struct {
	ID        string
	Database  string
	Schema    string
	CreatedAt time.Time
	Status    TxStatus
}

type TxInfo struct {
	ID        string    `json:"id"`
	Database  string    `json:"database"`
	Schema    string    `json:"schema"`
	Status    string    `json:"status"`
	Operation int       `json:"operation_count"`
	CreatedAt time.Time `json:"created_at"`
}

type Operation struct {
	ID            int             `db:"id"`
	TransactionID string          `db:"transaction_id"`
	Operation     string          `db:"operation"`
	TableName     string          `db:"table_name"`
	SQLQuery      string          `db:"sql_query"`
	Params        json.RawMessage `db:"params"`
	CreatedAt     time.Time       `db:"created_at"`
}

// Manager handles distributed transactions using the Saga pattern.
// Transaction metadata is stored in the main database (db).
// Operations are executed on the target database during commit.
type Manager struct {
	db   *sqlx.DB
	ttl  time.Duration
	stop chan struct{}
}

func NewManager(db *sqlx.DB, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	m := &Manager{
		db:   db,
		ttl:  ttl,
		stop: make(chan struct{}),
	}
	go m.cleanup()
	return m
}

func (m *Manager) Stop() {
	close(m.stop)
}

func (m *Manager) Start(ctx context.Context, database, schema string) (string, error) {
	id := generateID()

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO prest_transactions (id, database_name, schema_name, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', NOW(), NOW())`,
		id, database, schema)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	slog.Info("transaction started", "id", id, "database", database, "schema", schema)
	return id, nil
}

func (m *Manager) Get(ctx context.Context, txID string) (*ManagedTx, bool) {
	var tx ManagedTx
	err := m.db.GetContext(ctx, &tx,
		`SELECT id, database_name as database, schema_name as schema, created_at, 
		        CASE status WHEN 'pending' THEN 0 WHEN 'committed' THEN 1 WHEN 'rolled_back' THEN 2 END as status
		 FROM prest_transactions WHERE id = $1`, txID)
	if err != nil {
		return nil, false
	}
	if tx.Status != TxStatusPending {
		return nil, false
	}
	return &tx, true
}

func (m *Manager) AddOperation(ctx context.Context, txID, operation, tableName, sqlQuery string, params interface{}) error {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	_, err := m.db.ExecContext(ctx,
		`INSERT INTO prest_transaction_operations (transaction_id, operation, table_name, sql_query, params, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		txID, operation, tableName, sqlQuery, paramsJSON)
	if err != nil {
		return fmt.Errorf("failed to add operation: %w", err)
	}

	return nil
}

func (m *Manager) GetOperations(ctx context.Context, txID string) ([]Operation, error) {
	var ops []Operation
	err := m.db.SelectContext(ctx, &ops,
		`SELECT id, transaction_id, operation, table_name, sql_query, params, created_at
		 FROM prest_transaction_operations
		 WHERE transaction_id = $1
		 ORDER BY id`, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get operations: %w", err)
	}
	return ops, nil
}

// Commit executes all staged operations atomically on the target database.
// The target database is resolved via the adapter registry using the transaction's database name.
func (m *Manager) Commit(ctx context.Context, txID string, registry adapters.Registry) error {
	// Lock the transaction in the main database to prevent concurrent commits
	metaTx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin metadata transaction: %w", err)
	}
	defer metaTx.Rollback()

	var status, database string
	err = metaTx.QueryRowContext(ctx,
		`SELECT status, database_name FROM prest_transactions WHERE id = $1 FOR UPDATE`, txID).Scan(&status, &database)
	if err != nil {
		return fmt.Errorf("transaction not found: %s", txID)
	}
	if status != "pending" {
		return fmt.Errorf("transaction %s is not pending (status: %s)", txID, status)
	}

	// Get operations from the main database
	ops, err := m.getOpsForCommit(ctx, metaTx, txID)
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		return fmt.Errorf("transaction %s has no operations", txID)
	}

	// Get the target database adapter from the registry
	adapter, err := registry.Get(database)
	if err != nil {
		return fmt.Errorf("database %q not found in adapter registry: %w", database, err)
	}

	// Get the database connection for the target
	targetDB, err := postgres.DB(adapter)
	if err != nil {
		return fmt.Errorf("failed to get database connection for %q: %w", database, err)
	}

	// Execute all operations on the target database in a single transaction
	targetTx, err := targetDB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin target transaction on %q: %w", database, err)
	}
	defer targetTx.Rollback()

	for _, op := range ops {
		var params []interface{}
		if op.Params != nil {
			if err := json.Unmarshal(op.Params, &params); err != nil {
				return fmt.Errorf("failed to unmarshal params for operation %d: %w", op.ID, err)
			}
		}

		if len(params) > 0 {
			_, err = targetTx.ExecContext(ctx, op.SQLQuery, params...)
		} else {
			_, err = targetTx.ExecContext(ctx, op.SQLQuery)
		}
		if err != nil {
			return fmt.Errorf("failed to execute operation %d (%s on %s): %w", op.ID, op.Operation, op.TableName, err)
		}
	}

	// Commit the target database transaction
	if err := targetTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit on target database %q: %w", database, err)
	}

	// Update status and clean up in the main database
	_, err = metaTx.ExecContext(ctx,
		`UPDATE prest_transactions SET status = 'committed', updated_at = NOW() WHERE id = $1`, txID)
	if err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	_, err = metaTx.ExecContext(ctx,
		`DELETE FROM prest_transaction_operations WHERE transaction_id = $1`, txID)
	if err != nil {
		return fmt.Errorf("failed to clean up operations: %w", err)
	}

	if err := metaTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit metadata transaction: %w", err)
	}

	slog.Info("transaction committed", "id", txID, "database", database, "operations", len(ops))
	return nil
}

func (m *Manager) getOpsForCommit(ctx context.Context, tx *sqlx.Tx, txID string) ([]Operation, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, transaction_id, operation, table_name, sql_query, params, created_at
		 FROM prest_transaction_operations
		 WHERE transaction_id = $1
		 ORDER BY id`, txID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []Operation
	for rows.Next() {
		var op Operation
		if err := rows.Scan(&op.ID, &op.TransactionID, &op.Operation, &op.TableName, &op.SQLQuery, &op.Params, &op.CreatedAt); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (m *Manager) Rollback(ctx context.Context, txID string) error {
	result, err := m.db.ExecContext(ctx,
		`DELETE FROM prest_transactions WHERE id = $1 AND status = 'pending'`, txID)
	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("transaction %s not found or not pending", txID)
	}

	slog.Info("transaction rolled back", "id", txID)
	return nil
}

func (m *Manager) List(ctx context.Context, database, schema string) ([]TxInfo, error) {
	var result []TxInfo
	err := m.db.SelectContext(ctx, &result,
		`SELECT t.id, t.database_name as database, t.schema_name as schema, t.status,
		        (SELECT COUNT(*) FROM prest_transaction_operations WHERE transaction_id = t.id) as operation,
		        t.created_at
		 FROM prest_transactions t
		 WHERE t.database_name = $1 AND t.schema_name = $2 AND t.status = 'pending'
		 ORDER BY t.created_at`, database, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}
	return result, nil
}

func (m *Manager) Status(ctx context.Context, txID string) (*TxInfo, error) {
	var info TxInfo
	err := m.db.GetContext(ctx, &info,
		`SELECT t.id, t.database_name as database, t.schema_name as schema, t.status,
		        (SELECT COUNT(*) FROM prest_transaction_operations WHERE transaction_id = t.id) as operation,
		        t.created_at
		 FROM prest_transactions t
		 WHERE t.id = $1`, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}
	return &info, nil
}

func (m *Manager) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.cleanupExpired()
		}
	}
}

func (m *Manager) cleanupExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := m.db.ExecContext(ctx,
		`DELETE FROM prest_transactions 
		 WHERE status = 'pending' AND created_at < NOW() - INTERVAL '1 minute' * $1`,
		int(m.ttl.Minutes()))
	if err != nil {
		slog.Error("failed to cleanup expired transactions", "err", err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		slog.Info("cleaned up expired transactions", "count", rows)
	}
}

func generateID() string {
	return uuid.New().String()
}

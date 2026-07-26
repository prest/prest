package app

import (
	"github.com/jmoiron/sqlx"
)

const createTransactionTablesSQL = `
CREATE TABLE IF NOT EXISTS prest_transactions (
    id VARCHAR(36) PRIMARY KEY,
    database_name VARCHAR(255) NOT NULL,
    schema_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prest_transactions_status ON prest_transactions(status);
CREATE INDEX IF NOT EXISTS idx_prest_transactions_database_schema ON prest_transactions(database_name, schema_name);
CREATE INDEX IF NOT EXISTS idx_prest_transactions_created_at ON prest_transactions(created_at);

CREATE TABLE IF NOT EXISTS prest_transaction_operations (
    id SERIAL PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES prest_transactions(id) ON DELETE CASCADE,
    operation VARCHAR(10) NOT NULL,
    table_name TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    params JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prest_transaction_operations_tx_id ON prest_transaction_operations(transaction_id);
`

func EnsureTransactionTables(db *sqlx.DB) error {
	_, err := db.Exec(createTransactionTablesSQL)
	return err
}

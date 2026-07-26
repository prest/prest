package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/prest/prest/v2/internal/logsafe"

	"github.com/jmoiron/sqlx"
)

// Stmt statement representation
type Stmt struct {
	Mtx        *sync.Mutex
	PrepareMap map[string]map[string]*sql.Stmt
	pgCache    bool
}

func (p *postgres) getStmts() *Stmt {
	p.stmtsMu.Lock()
	defer p.stmtsMu.Unlock()
	if p.stmts == nil {
		p.stmts = &Stmt{
			Mtx:        &sync.Mutex{},
			PrepareMap: make(map[string]map[string]*sql.Stmt),
			pgCache:    p.cfg.PGCache,
		}
	}
	return p.stmts
}

// ClearStmt used to reset the cache and allow multiple tests
func (p *postgres) ClearStmt() {
	p.stmtsMu.Lock()
	defer p.stmtsMu.Unlock()
	p.stmts = nil
}

// GetStmt get statement cache (for tests).
func (p *postgres) GetStmt() *Stmt {
	return p.getStmts()
}

// Prepare statement.
// SQL passed here is assembled by the adapter from HTTP requests: identifiers and
// operators are validated (ident.IsValid, GetQueryOperator) and filter values use
// $n placeholders. pREST is a PostgREST-style query surface by design.
func (s *Stmt) Prepare(dbKey string, db *sqlx.DB, tx *sql.Tx, SQL string) (statement *sql.Stmt, err error) {
	if s.pgCache && (tx == nil) {
		var exists bool
		s.Mtx.Lock()
		if dbMap := s.PrepareMap[dbKey]; dbMap != nil {
			statement, exists = dbMap[SQL]
		}
		s.Mtx.Unlock()
		if exists {
			return
		}
	}

	if tx != nil {
		statement, err = tx.Prepare(SQL)
	} else {
		statement, err = db.Prepare(SQL)
	}

	if err != nil {
		return
	}
	if s.pgCache && (tx == nil) {
		s.Mtx.Lock()
		if dbMap := s.PrepareMap[dbKey]; dbMap != nil {
			if cached, ok := dbMap[SQL]; ok {
				s.Mtx.Unlock()
				_ = statement.Close()
				return cached, nil
			}
		} else {
			s.PrepareMap[dbKey] = make(map[string]*sql.Stmt)
		}
		s.PrepareMap[dbKey][SQL] = statement
		s.Mtx.Unlock()
	}
	return
}

// PrepareContext statement with context for cancellation/deadline support.
func (s *Stmt) PrepareContext(ctx context.Context, dbKey string, db *sqlx.DB, tx *sql.Tx, SQL string) (statement *sql.Stmt, err error) {
	if s.pgCache && (tx == nil) {
		var exists bool
		s.Mtx.Lock()
		if dbMap := s.PrepareMap[dbKey]; dbMap != nil {
			statement, exists = dbMap[SQL]
		}
		s.Mtx.Unlock()
		if exists {
			return
		}
	}

	if tx != nil {
		statement, err = tx.PrepareContext(ctx, SQL)
	} else {
		statement, err = db.PrepareContext(ctx, SQL)
	}

	if err != nil {
		return
	}
	if s.pgCache && (tx == nil) {
		s.Mtx.Lock()
		if dbMap := s.PrepareMap[dbKey]; dbMap != nil {
			if cached, ok := dbMap[SQL]; ok {
				s.Mtx.Unlock()
				_ = statement.Close()
				return cached, nil
			}
		} else {
			s.PrepareMap[dbKey] = make(map[string]*sql.Stmt)
		}
		s.PrepareMap[dbKey][SQL] = statement
		s.Mtx.Unlock()
	}
	return
}

// Prepare statement func
func (p *postgres) Prepare(db *sqlx.DB, SQL string) (stmt *sql.Stmt, err error) {
	return p.getStmts().Prepare(p.conn.CacheKeyForDB(db), db, nil, SQL)
}

// PrepareContext statement func
func (p *postgres) PrepareContext(ctx context.Context, db *sqlx.DB, SQL string) (stmt *sql.Stmt, err error) {
	return p.getStmts().PrepareContext(ctx, p.conn.CacheKeyForDB(db), db, nil, SQL)
}

// PrepareTx statement func
func (p *postgres) PrepareTx(tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	return p.getStmts().Prepare("", nil, tx, SQL)
}

// PrepareTxContext statement func with context for cancellation/deadline support.
func (p *postgres) PrepareTxContext(ctx context.Context, tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	return p.getStmts().PrepareContext(ctx, "", nil, tx, SQL)
}

// GetTransaction get transaction
func (adapter *postgres) GetTransaction() (tx *sql.Tx, err error) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Info("log details", "err", logsafe.Error(err))
		return
	}
	return db.Begin()
}

// GetTransactionCtx get transaction
func (adapter *postgres) GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("error details", "err", logsafe.Error(err))
		return
	}
	return db.BeginTx(ctx, nil)
}

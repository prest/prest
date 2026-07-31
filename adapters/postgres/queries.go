package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/scanner"
	"github.com/prest/prest/v2/internal/logsafe"

	"log/slog"
)

// logFailedSQL emits the statement that failed at debug level only. Script
// templates interpolate request headers and query parameters straight into the
// SQL text, so a rendered statement carries caller-supplied data; keeping it off
// the always-on error path means an operator opts in to seeing it.
// codeql[go/clear-text-logging]
func logFailedSQL(sql string) {
	slog.Debug("failed sql", "sql", sql)
}

// WriteSQL perform INSERT's, UPDATE's, DELETE's operations
func (adapter *postgres) WriteSQL(sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("connection get error", "err", logsafe.Error(err))
		sc = &scanner.PrestScanner{Error: fmt.Errorf("connection get error: %w", err)}
		return
	}
	stmt, err := adapter.Prepare(db, sql)
	if err != nil {
		slog.Error("could not prepare sql", "err", err)
		logFailedSQL(sql)
		sc = &scanner.PrestScanner{Error: fmt.Errorf("could not prepare sql: %w", err)}
		return
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.Exec(valuesAux...)
	if err != nil {
		slog.Error("could not execute sql", "err", logsafe.Error(err))
		logFailedSQL(sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
	return
}

// WriteSQLCtx perform INSERT's, UPDATE's, DELETE's operations
func (adapter *postgres) WriteSQLCtx(ctx context.Context, sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("connection get error", "err", logsafe.Error(err))
		sc = &scanner.PrestScanner{Error: fmt.Errorf("connection get error: %w", err)}
		return
	}
	stmt, err := adapter.PrepareContext(ctx, db, sql)
	if err != nil {
		slog.Error("could not prepare sql", "err", logsafe.Error(err))
		logFailedSQL(sql)
		sc = &scanner.PrestScanner{Error: fmt.Errorf("could not prepare sql: %w", err)}
		return
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.ExecContext(ctx, valuesAux...)
	if err != nil {
		slog.Error("could not execute sql", "err", logsafe.Error(err))
		logFailedSQL(sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
	return
}

// ExecuteScripts run sql templates created by users
func (adapter *postgres) ExecuteScripts(method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return adapter.Query(sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return adapter.WriteSQL(sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}

// ExecuteScriptsCtx run sql templates created by users
func (adapter *postgres) ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return adapter.QueryCtx(ctx, sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return adapter.WriteSQLCtx(ctx, sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}

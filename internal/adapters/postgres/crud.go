package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/scanner"
	"github.com/prest/prest/v2/internal/template"
	"github.com/prest/prest/v2/pkg/adapters"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// QueryCtx process queries using the DB name from Context
//
// allows setting timeout
func (adapter *postgres) QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	// use the db_name that was set on request to avoid runtime collisions
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	SQL = fmt.Sprintf("SELECT %s(s) FROM (%s) s", adapter.cfg.JSONAggType, SQL)
	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	p, err := adapter.Prepare(db, SQL)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	var jsonData []byte
	err = p.QueryRowContext(ctx, params...).Scan(&jsonData)
	if len(jsonData) == 0 {
		jsonData = []byte("[]")
	}
	return &scanner.PrestScanner{
		Error:   err,
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

func (adapter *postgres) Query(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Info("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	SQL = fmt.Sprintf("SELECT %s(s) FROM (%s) s", adapter.cfg.JSONAggType, SQL)
	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	p, err := adapter.Prepare(db, SQL)
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	var jsonData []byte
	err = p.QueryRow(params...).Scan(&jsonData)
	if len(jsonData) == 0 {
		jsonData = []byte("[]")
	}
	return &scanner.PrestScanner{
		Error:   err,
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

// QueryCount process queries with count
func (adapter *postgres) QueryCount(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}

	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	p, err := adapter.Prepare(db, SQL)
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := p.QueryRow(params...)
	if err = row.Scan(&result.Count); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	var byt []byte
	byt, err = json.Marshal(result)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(byt),
	}
}

// QueryCountCtx process queries with count
func (adapter *postgres) QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	p, err := adapter.Prepare(db, SQL)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := p.QueryRowContext(ctx, params...)
	if err = row.Scan(&result.Count); err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	var byt []byte
	byt, err = json.Marshal(result)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(byt),
	}
}

// PaginateIfPossible when passing non-valid paging parameters (conversion to integer) the query will be made with default value
func (adapter *postgres) PaginateIfPossible(r *http.Request) (paginatedQuery string, err error) {
	values := r.URL.Query()
	if _, ok := values[pageNumberKey]; !ok {
		paginatedQuery = ""
		return
	}
	pageNumber, err := strconv.Atoi(values[pageNumberKey][0])
	if err != nil {
		return
	}
	pageSize := defaultPageSize
	if size, ok := values[pageSizeKey]; ok {
		pageSize, err = strconv.Atoi(size[0])
		if err != nil {
			return
		}
	}
	return template.LimitOffset(fmt.Sprint(pageNumber), fmt.Sprint(pageSize))
}

// BatchInsertCopy execute batch insert sql into a table unsing copy
func (adapter *postgres) BatchInsertCopy(dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	tx, err := db.Begin()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	defer func() {
		var txerr error
		if err != nil {
			txerr = tx.Rollback()
			if txerr != nil {
				slog.Error("log details", "err", txerr)
				return
			}
			return
		}
		txerr = tx.Commit()
		if txerr != nil {
			slog.Error("log details", "err", txerr)
			return
		}
	}()
	for i := range keys {
		if strings.HasPrefix(keys[i], `"`) {
			keys[i], err = strconv.Unquote(keys[i])
			if err != nil {
				slog.Error("log details", "err", err)
				return &scanner.PrestScanner{Error: err}
			}
		}
	}
	stmt, err := tx.Prepare(pq.CopyInSchema(schema, table, keys...))
	if err != nil {
		slog.Info("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	initOffSet := 0
	limitOffset := len(keys)
	for limitOffset <= len(values) {
		_, err = stmt.Exec(values[initOffSet:limitOffset]...)
		if err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		initOffSet = limitOffset
		limitOffset += len(keys)
	}
	_, err = stmt.Exec()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	err = stmt.Close()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return &scanner.PrestScanner{}
}

// BatchInsertCopyCtx execute batch insert sql into a table unsing copy
func (adapter *postgres) BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	defer func() {
		var txerr error
		if err != nil {
			txerr = tx.Rollback()
			if txerr != nil {
				slog.Error("log details", "err", txerr)
				return
			}
			return
		}
		txerr = tx.Commit()
		if txerr != nil {
			slog.Error("log details", "err", txerr)
			return
		}
	}()
	for i := range keys {
		if strings.HasPrefix(keys[i], `"`) {
			keys[i], err = strconv.Unquote(keys[i])
			if err != nil {
				slog.Error("log details", "err", err)
				return &scanner.PrestScanner{Error: err}
			}
		}
	}
	stmt, err := tx.PrepareContext(ctx, pq.CopyInSchema(schema, table, keys...))
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	initOffSet := 0
	limitOffset := len(keys)
	for limitOffset <= len(values) {
		_, err = stmt.ExecContext(ctx, values[initOffSet:limitOffset]...)
		if err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		initOffSet = limitOffset
		limitOffset += len(keys)
	}
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	err = stmt.Close()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return &scanner.PrestScanner{}
}

// BatchInsertValues execute batch insert sql into a table unsing multi values
func (adapter *postgres) BatchInsertValues(SQL string, values ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := adapter.fullInsert(context.Background(), db, nil, SQL)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	jsonData := []byte("[")
	rows, err := stmt.Query(values...)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	for rows.Next() {
		if err = rows.Err(); err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		if !bytes.Equal(jsonData, []byte("[")) {
			obj := fmt.Sprintf("%s,%s", jsonData, data)
			jsonData = []byte(obj)
			continue
		}
		jsonData = append(jsonData, data...)
	}
	jsonData = append(jsonData, byte(']'))
	return &scanner.PrestScanner{
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

// BatchInsertValuesCtx execute batch insert sql into a table unsing multi values
func (adapter *postgres) BatchInsertValuesCtx(ctx context.Context, SQL string, values ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := adapter.fullInsert(ctx, db, nil, SQL)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	jsonData := []byte("[")
	rows, err := stmt.QueryContext(ctx, values...)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	for rows.Next() {
		if err = rows.Err(); err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			slog.Error("log details", "err", err)
			return &scanner.PrestScanner{Error: err}
		}
		if !bytes.Equal(jsonData, []byte("[")) {
			obj := fmt.Sprintf("%s,%s", jsonData, data)
			jsonData = []byte(obj)
			continue
		}
		jsonData = append(jsonData, data...)
	}
	jsonData = append(jsonData, byte(']'))
	return &scanner.PrestScanner{
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

func (adapter *postgres) fullInsert(ctx context.Context, db *sqlx.DB, tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	tableName := insertTableNameQuotesRegex.FindStringSubmatch(SQL)
	if len(tableName) < 2 {
		tableName = insertTableNameRegex.FindStringSubmatch(SQL)
		if len(tableName) < 2 {
			err = ErrNoTableName
			return
		}
	}
	SQL = fmt.Sprintf(`%s RETURNING row_to_json("%s")`, SQL, tableName[2])
	if tx != nil {
		if ctx != nil {
			return adapter.PrepareTxContext(ctx, tx, SQL)
		}
		return adapter.PrepareTx(tx, SQL)
	}
	if ctx != nil {
		return adapter.PrepareContext(ctx, db, SQL)
	}
	return adapter.Prepare(db, SQL)
}

// Insert execute insert sql into a table
func (adapter *postgres) Insert(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.insert(nil, db, nil, SQL, params...)
}

// InsertCtx execute insert sql into a table
func (adapter *postgres) InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.insert(ctx, db, nil, SQL, params...)
}

// InsertWithTransaction execute insert sql into a table
func (adapter *postgres) InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.insert(nil, nil, tx, SQL, params...)
}

func (adapter *postgres) insert(ctx context.Context, db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	stmt, err := adapter.fullInsert(ctx, db, tx, SQL)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	slog.Debug("log details", "sql", SQL, "parameters", params)
	var jsonData []byte
	if ctx != nil {
		err = stmt.QueryRowContext(ctx, params...).Scan(&jsonData)
	} else {
		err = stmt.QueryRow(params...).Scan(&jsonData)
	}
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
}

// Delete execute delete sql into a table
func (adapter *postgres) Delete(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.delete(nil, db, nil, SQL, params...)
}

// DeleteCtx execute delete sql into a table
func (adapter *postgres) DeleteCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.delete(ctx, db, nil, SQL, params...)
}

// DeleteWithTransaction execute delete sql into a table
func (adapter *postgres) DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.delete(nil, nil, tx, SQL, params...)
}

func (adapter *postgres) delete(ctx context.Context, db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	var stmt *sql.Stmt
	var err error
	if tx != nil {
		if ctx != nil {
			stmt, err = adapter.PrepareTxContext(ctx, tx, SQL)
		} else {
			stmt, err = adapter.PrepareTx(tx, SQL)
		}
	} else if ctx != nil {
		stmt, err = adapter.PrepareContext(ctx, db, SQL)
	} else {
		stmt, err = adapter.Prepare(db, SQL)
	}
	if err != nil {
		slog.Error("could not prepare sql", "sql", SQL, "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	if strings.Contains(SQL, "RETURNING") {
		var rows *sql.Rows
		if ctx != nil {
			rows, _ = stmt.QueryContext(ctx, params...)
		} else {
			rows, _ = stmt.Query(params...)
		}
		cols, _ := rows.Columns()
		var data []map[string]interface{}
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}
			if err := rows.Scan(columnPointers...); err != nil {
				slog.Error("row scan error", "err", err)
				os.Exit(1)
			}
			m := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				switch (*val).(type) {
				case []uint8:
					m[colName] = string((*val).([]byte))
				default:
					m[colName] = *val
				}
			}
			data = append(data, m)
		}
		jsonData, _ := json.Marshal(data)
		return &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
	}
	var result sql.Result
	var rowsAffected int64
	if ctx != nil {
		result, err = stmt.ExecContext(ctx, params...)
	} else {
		result, err = stmt.Exec(params...)
	}
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
}

// Update execute update sql into a table
func (adapter *postgres) Update(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.conn.Get()
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.update(nil, db, nil, SQL, params...)
}

// UpdateCtx execute update sql into a table
func (adapter *postgres) UpdateCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		slog.Error("log details", "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.update(ctx, db, nil, SQL, params...)
}

// UpdateWithTransaction execute update sql into a table
func (adapter *postgres) UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.update(nil, nil, tx, SQL, params...)
}

func (adapter *postgres) update(ctx context.Context, db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	var stmt *sql.Stmt
	var err error
	if tx != nil {
		if ctx != nil {
			stmt, err = adapter.PrepareTxContext(ctx, tx, SQL)
		} else {
			stmt, err = adapter.PrepareTx(tx, SQL)
		}
	} else if ctx != nil {
		stmt, err = adapter.PrepareContext(ctx, db, SQL)
	} else {
		stmt, err = adapter.Prepare(db, SQL)
	}
	if err != nil {
		slog.Error("could not prepare sql", "sql", SQL, "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	slog.Debug("generated SQL", "sql", SQL, "parameters", params)
	if strings.Contains(SQL, "RETURNING") {
		var rows *sql.Rows
		if ctx != nil {
			rows, _ = stmt.QueryContext(ctx, params...)
		} else {
			rows, _ = stmt.Query(params...)
		}
		cols, _ := rows.Columns()
		var data []map[string]interface{}
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}
			if err := rows.Scan(columnPointers...); err != nil {
				slog.Error("row scan error", "err", err)
				os.Exit(1)
			}
			m := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				switch (*val).(type) {
				case []uint8:
					m[colName] = string((*val).([]byte))
				default:
					m[colName] = *val
				}
			}
			data = append(data, m)
		}
		jsonData, _ := json.Marshal(data)
		return &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
	}
	var result sql.Result
	var rowsAffected int64
	if ctx != nil {
		result, err = stmt.ExecContext(ctx, params...)
	} else {
		result, err = stmt.Exec(params...)
	}
	if err != nil {
		slog.Error("could not execute sql", "sql", SQL, "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		slog.Error("could not get rows affected", "sql", SQL, "err", err)
		return &scanner.PrestScanner{Error: err}
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
}

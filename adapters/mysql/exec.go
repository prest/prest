package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql/statements"
	"github.com/prest/prest/v2/adapters/scanner"
)

func scanErr(err error) adapters.Scanner {
	return &scanner.PrestScanner{Error: wrapDriver(err)}
}

func scanBuf(buf []byte, err error, query bool) adapters.Scanner {
	if buf == nil && err == nil {
		buf = []byte("[]")
	}
	return &scanner.PrestScanner{Buff: bytes.NewBuffer(buf), Error: wrapDriver(err), IsQuery: query}
}

func wrapDriver(err error) error {
	if err == nil {
		return nil
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1146 {
		return fmt.Errorf("%w: %s", adapters.ErrRelationNotFound, me.Message)
	}
	return err
}

func (a *Adapter) Query(sql string, params ...any) adapters.Scanner {
	return a.QueryCtx(context.Background(), sql, params...)
}

func (a *Adapter) QueryCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return scanErr(err)
	}
	rows, err := execQuery(ctx, db, query, params...)
	if err != nil {
		return scanErr(err)
	}
	defer rows.Close()
	buf, err := scanJSONArray(rows)
	return scanBuf(buf, err, true)
}

func (a *Adapter) QueryCount(sql string, params ...any) adapters.Scanner {
	return a.QueryCountCtx(context.Background(), sql, params...)
}

func (a *Adapter) QueryCountCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return scanErr(err)
	}
	var n int64
	if err := execQueryRow(ctx, db, query, params...).Scan(&n); err != nil {
		return scanErr(err)
	}
	buf, err := json.Marshal(map[string]int64{"count": n})
	return scanBuf(buf, err, false)
}

func (a *Adapter) Insert(query string, params ...any) adapters.Scanner {
	return a.InsertCtx(context.Background(), query, params...)
}

func (a *Adapter) InsertCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	return a.insert(ctx, nil, query, params...)
}

func (a *Adapter) InsertWithTransaction(tx *sql.Tx, query string, params ...any) adapters.Scanner {
	return a.insert(context.Background(), tx, query, params...)
}

func (a *Adapter) Update(query string, params ...any) adapters.Scanner {
	return a.UpdateCtx(context.Background(), query, params...)
}

func (a *Adapter) UpdateCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	return a.update(ctx, nil, query, params...)
}

func (a *Adapter) UpdateWithTransaction(tx *sql.Tx, query string, params ...any) adapters.Scanner {
	return a.update(context.Background(), tx, query, params...)
}

func (a *Adapter) Delete(query string, params ...any) adapters.Scanner {
	return a.DeleteCtx(context.Background(), query, params...)
}

func (a *Adapter) DeleteCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	return a.delete(ctx, nil, query, params...)
}

func (a *Adapter) DeleteWithTransaction(tx *sql.Tx, query string, params ...any) adapters.Scanner {
	return a.delete(context.Background(), tx, query, params...)
}

func (a *Adapter) BatchInsertValues(query string, params ...any) adapters.Scanner {
	return a.BatchInsertValuesCtx(context.Background(), query, params...)
}

func (a *Adapter) BatchInsertValuesCtx(ctx context.Context, query string, params ...any) adapters.Scanner {
	return a.batchInsert(ctx, query, params...)
}

func (a *Adapter) BatchInsertCopy(dbname, schema, table string, keys []string, params ...any) adapters.Scanner {
	return a.BatchInsertCopyCtx(context.Background(), dbname, schema, table, keys, params...)
}

func (a *Adapter) BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, params ...any) adapters.Scanner {
	if len(keys) == 0 {
		return scanErr(errBodyEmpty)
	}
	quoted := make([]string, len(keys))
	for i, key := range keys {
		key = strings.Trim(key, "`")
		q, err := quoteIdent(key)
		if err != nil {
			return scanErr(err)
		}
		quoted[i] = q
	}
	if len(params)%len(keys) != 0 {
		return scanErr(fmt.Errorf("batch values do not match columns"))
	}
	rows := len(params) / len(keys)
	groups := make([]string, rows)
	for i := 0; i < rows; i++ {
		ph, err := placeholders(1, len(keys))
		if err != nil {
			return scanErr(err)
		}
		groups[i] = ph
	}
	query := fmt.Sprintf("INSERT INTO %s(%s) VALUES%s", tableReference(schema, table), strings.Join(quoted, ","), strings.Join(groups, ","))
	return a.batchInsert(ctx, query, params...)
}

func (a *Adapter) ShowTable(schema, table string) adapters.Scanner {
	return a.ShowTableCtx(context.Background(), schema, table)
}

func (a *Adapter) ShowTableCtx(ctx context.Context, schema, table string) adapters.Scanner {
	query := statements.ShowColumns + statements.ShowTableWhere
	return a.QueryCtx(ctx, query, table, schema)
}

func (a *Adapter) ShowColumnsCtx(ctx context.Context) adapters.Scanner {
	return a.QueryCtx(ctx, statements.ShowColumns+statements.ShowColumnsWhere)
}

func (a *Adapter) ExecuteScripts(method, query string, values []any) adapters.Scanner {
	return a.ExecuteScriptsCtx(context.Background(), method, query, values)
}

func (a *Adapter) ExecuteScriptsCtx(ctx context.Context, method, query string, values []any) adapters.Scanner {
	switch method {
	case "GET":
		return a.QueryCtx(ctx, query, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return a.execRowsAffected(ctx, nil, query, values...)
	default:
		return scanErr(fmt.Errorf("invalid method %s", method))
	}
}

func (a *Adapter) execRowsAffected(ctx context.Context, tx *sql.Tx, query string, params ...any) adapters.Scanner {
	var (
		res sql.Result
		err error
	)
	if tx != nil {
		res, err = execStmt(ctx, tx, query, params...)
	} else {
		db, derr := a.dbFromCtx(ctx)
		if derr != nil {
			return scanErr(derr)
		}
		res, err = execStmt(ctx, db, query, params...)
	}
	if err != nil {
		return scanErr(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return scanErr(err)
	}
	buf, err := json.Marshal(map[string]int64{"rows_affected": n})
	return scanBuf(buf, err, false)
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func execQuery(ctx context.Context, q queryer, query string, args ...any) (*sql.Rows, error) {
	// Adapter-built SQL: identifiers are quoted, values are bound arguments.
	// codeql[go/sql-injection]
	return q.QueryContext(ctx, query, args...)
}

func execQueryRow(ctx context.Context, q queryer, query string, args ...any) *sql.Row {
	// Adapter-built SQL: identifiers are quoted, values are bound arguments.
	// codeql[go/sql-injection]
	return q.QueryRowContext(ctx, query, args...)
}

func execStmt(ctx context.Context, q queryer, query string, args ...any) (sql.Result, error) {
	// Adapter-built SQL: identifiers are quoted, values are bound arguments.
	// codeql[go/sql-injection]
	return q.ExecContext(ctx, query, args...)
}

package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql/statements"
)

type pkColumn struct {
	name          string
	autoIncrement bool
}

var (
	insertHeadRe = regexp.MustCompile("(?is)^\\s*INSERT\\s+INTO\\s+(.+?)\\s*\\((.*?)\\)\\s*VALUES")
	updateHeadRe = regexp.MustCompile("(?is)^\\s*UPDATE\\s+(.+?)\\s+SET\\s+")
	deleteHeadRe = regexp.MustCompile("(?is)^\\s*DELETE\\s+FROM\\s+(.+?)(?:\\s+WHERE\\s+|\\s*$)")
)

func splitReturning(query string) (string, string, bool) {
	idx := strings.LastIndex(query, " RETURNING ")
	if idx < 0 {
		return query, "", false
	}
	return strings.TrimSpace(query[:idx]), strings.TrimSpace(query[idx+len(" RETURNING "):]), true
}

func (a *Adapter) insert(ctx context.Context, tx *sql.Tx, query string, params ...any) adapters.Scanner {
	ownTx := tx == nil
	var err error
	if ownTx {
		tx, err = a.GetTransactionCtx(ctx)
		if err != nil {
			return scanErr(err)
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback()
			}
		}()
	}
	res, err := tx.ExecContext(ctx, query, params...)
	if err != nil {
		return scanErr(err)
	}
	buf, err := a.insertedObject(ctx, tx, query, params, res)
	if err != nil {
		return scanErr(err)
	}
	if ownTx {
		if cerr := tx.Commit(); cerr != nil {
			err = cerr
			return scanErr(cerr)
		}
	}
	return scanBuf(buf, nil, false)
}

func (a *Adapter) insertedObject(ctx context.Context, tx *sql.Tx, query string, params []any, res sql.Result) ([]byte, error) {
	schema, table, cols, err := parseInsert(query)
	if err != nil {
		return insertedFallback(cols, params, res)
	}
	pks, err := a.primaryKeys(ctx, tx, schema, table)
	if err != nil {
		return nil, err
	}
	where, args, ok := pkWhere(pks, cols, params, res)
	if !ok {
		return insertedFallback(cols, params, res)
	}
	q := fmt.Sprintf("SELECT * FROM %s.%s WHERE %s", mustQuote(schema), mustQuote(table), where)
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONObject(rows)
}

func insertedFallback(cols []string, params []any, res sql.Result) ([]byte, error) {
	obj := map[string]any{}
	for i, col := range cols {
		if i < len(params) {
			obj[strings.Trim(col, "`")] = params[i]
		}
	}
	if res != nil {
		if id, err := res.LastInsertId(); err == nil && id != 0 {
			obj["last_insert_id"] = id
		}
	}
	return json.Marshal(obj)
}

func parseInsert(query string) (schema, table string, cols []string, err error) {
	m := insertHeadRe.FindStringSubmatch(query)
	if len(m) < 3 {
		return "", "", nil, fmt.Errorf("cannot parse insert")
	}
	schema, table = splitTableRef(m[1])
	for _, col := range strings.Split(m[2], ",") {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "`")
		if col != "" {
			cols = append(cols, col)
		}
	}
	return schema, table, cols, nil
}

func splitTableRef(ref string) (schema, table string) {
	ref = strings.TrimSpace(ref)
	parts := strings.Split(ref, ".")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], "`")
	}
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return "", ""
}

func (a *Adapter) primaryKeys(ctx context.Context, tx *sql.Tx, schema, table string) ([]pkColumn, error) {
	key := schema + "." + table
	a.pkMu.Lock()
	defer a.pkMu.Unlock()
	if a.pkCache == nil {
		a.pkCache = map[string][]pkColumn{}
	}
	if cached, ok := a.pkCache[key]; ok {
		return cached, nil
	}
	rows, err := tx.QueryContext(ctx, statements.PKColumns, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []pkColumn
	for rows.Next() {
		var name, extra string
		if err := rows.Scan(&name, &extra); err != nil {
			return nil, err
		}
		cols = append(cols, pkColumn{
			name:          name,
			autoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	a.pkCache[key] = cols
	return cols, nil
}

func pkWhere(pks []pkColumn, cols []string, params []any, res sql.Result) (string, []any, bool) {
	if len(pks) == 0 {
		return "", nil, false
	}
	values := map[string]any{}
	for i, col := range cols {
		if i < len(params) {
			values[col] = params[i]
		}
	}
	var lastID int64
	if res != nil {
		lastID, _ = res.LastInsertId()
	}
	parts := make([]string, 0, len(pks))
	args := make([]any, 0, len(pks))
	for _, pk := range pks {
		if v, ok := values[pk.name]; ok {
			parts = append(parts, mustQuote(pk.name)+"= ?")
			args = append(args, v)
			continue
		}
		if pk.autoIncrement && lastID != 0 {
			parts = append(parts, mustQuote(pk.name)+"= ?")
			args = append(args, lastID)
			continue
		}
		return "", nil, false
	}
	return strings.Join(parts, " AND "), args, true
}

func (a *Adapter) update(ctx context.Context, tx *sql.Tx, query string, params ...any) adapters.Scanner {
	stmt, returning, has := splitReturning(query)
	if !has {
		return a.execRowsAffected(ctx, tx, stmt, params...)
	}
	ownTx := tx == nil
	var err error
	if ownTx {
		tx, err = a.GetTransactionCtx(ctx)
		if err != nil {
			return scanErr(err)
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback()
			}
		}()
	}
	setSQL, whereSQL := splitWhere(stmt)
	setN := strings.Count(setSQL, "?")
	if setN > len(params) {
		err = fmt.Errorf("update placeholders exceed values")
		return scanErr(err)
	}
	if _, err = tx.ExecContext(ctx, stmt, params...); err != nil {
		return scanErr(err)
	}
	table := updateTable(stmt)
	sel := fmt.Sprintf("SELECT %s FROM %s", returning, table)
	whereArgs := params[setN:]
	if whereSQL != "" {
		sel += " WHERE " + whereSQL
	}
	rows, err := tx.QueryContext(ctx, sel, whereArgs...)
	if err != nil {
		return scanErr(err)
	}
	defer rows.Close()
	buf, err := scanJSONArray(rows)
	if err != nil {
		return scanErr(err)
	}
	if ownTx {
		if cerr := tx.Commit(); cerr != nil {
			return scanErr(cerr)
		}
	}
	return scanBuf(buf, nil, true)
}

func (a *Adapter) delete(ctx context.Context, tx *sql.Tx, query string, params ...any) adapters.Scanner {
	stmt, returning, has := splitReturning(query)
	if !has {
		return a.execRowsAffected(ctx, tx, stmt, params...)
	}
	ownTx := tx == nil
	var err error
	if ownTx {
		tx, err = a.GetTransactionCtx(ctx)
		if err != nil {
			return scanErr(err)
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback()
			}
		}()
	}
	_, whereSQL := splitWhere(stmt)
	table := deleteTable(stmt)
	sel := fmt.Sprintf("SELECT %s FROM %s", returning, table)
	if whereSQL != "" {
		sel += " WHERE " + whereSQL
	}
	rows, err := tx.QueryContext(ctx, sel, params...)
	if err != nil {
		return scanErr(err)
	}
	buf, err := scanJSONArray(rows)
	rows.Close()
	if err != nil {
		return scanErr(err)
	}
	if _, err = tx.ExecContext(ctx, stmt, params...); err != nil {
		return scanErr(err)
	}
	if ownTx {
		if cerr := tx.Commit(); cerr != nil {
			return scanErr(cerr)
		}
	}
	return scanBuf(buf, nil, true)
}

func splitWhere(query string) (head, where string) {
	idx := strings.Index(query, " WHERE ")
	if idx < 0 {
		return query, ""
	}
	return query[:idx], query[idx+len(" WHERE "):]
}

func updateTable(query string) string {
	m := updateHeadRe.FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func deleteTable(query string) string {
	m := deleteHeadRe.FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func (a *Adapter) batchInsert(ctx context.Context, query string, params ...any) adapters.Scanner {
	tx, err := a.GetTransactionCtx(ctx)
	if err != nil {
		return scanErr(err)
	}
	res, err := tx.ExecContext(ctx, query, params...)
	if err != nil {
		_ = tx.Rollback()
		return scanErr(err)
	}
	schema, table, cols, perr := parseInsert(query)
	var buf []byte
	if perr == nil {
		buf, err = a.batchImages(ctx, tx, schema, table, cols, params, res)
	}
	if err != nil || buf == nil {
		buf, err = batchFallback(cols, params, res)
	}
	if err != nil {
		_ = tx.Rollback()
		return scanErr(err)
	}
	if err = tx.Commit(); err != nil {
		return scanErr(err)
	}
	return scanBuf(buf, nil, true)
}

func (a *Adapter) batchImages(ctx context.Context, tx *sql.Tx, schema, table string, cols []string, params []any, res sql.Result) ([]byte, error) {
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns")
	}
	pks, err := a.primaryKeys(ctx, tx, schema, table)
	if err != nil {
		return nil, err
	}
	if len(pks) != 1 {
		return nil, fmt.Errorf("batch image needs a single primary key")
	}
	pk := pks[0]
	n := len(params) / len(cols)
	if n == 0 {
		return []byte("[]"), nil
	}
	idx := -1
	for i, col := range cols {
		if col == pk.name {
			idx = i
			break
		}
	}
	args := make([]any, 0, n)
	if idx >= 0 {
		for row := 0; row < n; row++ {
			args = append(args, params[row*len(cols)+idx])
		}
	} else if pk.autoIncrement {
		id, err := res.LastInsertId()
		if err != nil || id == 0 {
			return nil, fmt.Errorf("missing last insert id")
		}
		for i := 0; i < n; i++ {
			args = append(args, id+int64(i))
		}
	} else {
		return nil, fmt.Errorf("primary key not in insert")
	}
	ph := make([]string, len(args))
	for i := range ph {
		ph[i] = "?"
	}
	q := fmt.Sprintf("SELECT * FROM %s.%s WHERE %s IN (%s)", mustQuote(schema), mustQuote(table), mustQuote(pk.name), strings.Join(ph, ","))
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONArray(rows)
}

func batchFallback(cols []string, params []any, res sql.Result) ([]byte, error) {
	if len(cols) == 0 {
		n := int64(0)
		if res != nil {
			n, _ = res.RowsAffected()
		}
		return json.Marshal([]map[string]int64{{"rows_affected": n}})
	}
	var last int64
	if res != nil {
		last, _ = res.LastInsertId()
	}
	n := len(params) / len(cols)
	out := make([]map[string]any, 0, n)
	for row := 0; row < n; row++ {
		obj := map[string]any{}
		for i, col := range cols {
			obj[col] = params[row*len(cols)+i]
		}
		if last != 0 {
			obj["last_insert_id"] = last + int64(row)
		}
		out = append(out, obj)
	}
	if out == nil {
		out = []map[string]any{}
	}
	return json.Marshal(out)
}

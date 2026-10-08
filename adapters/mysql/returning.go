package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	res, err := execStmt(ctx, tx, query, params...)
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
	rows, err := execQuery(ctx, tx, q, args...)
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
	if a.pkCache == nil {
		a.pkCache = map[string][]pkColumn{}
	}
	if cached, ok := a.pkCache[key]; ok {
		a.pkMu.Unlock()
		return cached, nil
	}
	a.pkMu.Unlock()

	rows, err := tx.QueryContext(ctx, statements.PKColumns, schema, table)
	if err != nil {
		return nil, err
	}
	cols, err := scanPKColumns(rows)
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(cols) == 0 && schema != "" {
		var one int64
		err = tx.QueryRowContext(ctx, statements.TableExists, schema, table).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s.%s", adapters.ErrRelationNotFound, schema, table)
		}
		if err != nil {
			return nil, err
		}
	}

	a.pkMu.Lock()
	if a.pkCache == nil {
		a.pkCache = map[string][]pkColumn{}
	}
	if cached, ok := a.pkCache[key]; ok {
		a.pkMu.Unlock()
		return cached, nil
	}
	a.pkCache[key] = cols
	a.pkMu.Unlock()
	return cols, nil
}

func scanPKColumns(rows *sql.Rows) ([]pkColumn, error) {
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
	table := updateTable(stmt)
	if table == "" {
		err = fmt.Errorf("update returning: empty table")
		return scanErr(err)
	}
	schema, tbl := splitTableRef(table)
	pks, err := a.primaryKeys(ctx, tx, schema, tbl)
	if err != nil {
		return scanErr(err)
	}
	if len(pks) == 0 {
		err = fmt.Errorf("update returning: no primary key")
		return scanErr(err)
	}
	if assignsPrimaryKey(setSQL, pks) {
		err = fmt.Errorf("update returning: cannot change primary key")
		return scanErr(err)
	}
	var whereArgs []any
	if whereSQL != "" {
		whereArgs = params[setN:]
	}
	keyRows, err := execQuery(ctx, tx, pkForUpdateSQL(pks, table, whereSQL), whereArgs...)
	if err != nil {
		return scanErr(err)
	}
	tuples, keyErr := scanKeyTuples(keyRows, len(pks))
	closeErr := keyRows.Close()
	if keyErr != nil {
		err = keyErr
		return scanErr(err)
	}
	if closeErr != nil {
		err = closeErr
		return scanErr(err)
	}
	if _, err = execStmt(ctx, tx, stmt, params...); err != nil {
		return scanErr(err)
	}
	if len(tuples) == 0 {
		if ownTx {
			if cerr := tx.Commit(); cerr != nil {
				return scanErr(cerr)
			}
		}
		return scanBuf([]byte("[]"), nil, true)
	}
	pred, predArgs := pkPredicate(pks, tuples)
	sel := fmt.Sprintf("SELECT %s FROM %s WHERE %s", returning, table, pred)
	rows, err := execQuery(ctx, tx, sel, predArgs...)
	if err != nil {
		return scanErr(err)
	}
	buf, err := scanJSONArray(rows)
	closeErr = rows.Close()
	if err != nil {
		return scanErr(err)
	}
	if closeErr != nil {
		err = closeErr
		return scanErr(err)
	}
	if ownTx {
		if cerr := tx.Commit(); cerr != nil {
			return scanErr(cerr)
		}
	}
	return scanBuf(buf, nil, true)
}

func assignsPrimaryKey(setSQL string, pks []pkColumn) bool {
	idx := strings.Index(strings.ToUpper(setSQL), " SET ")
	if idx < 0 {
		return false
	}
	tail := setSQL[idx+len(" SET "):]
	for _, part := range strings.Split(tail, ",") {
		lhs, _, _ := strings.Cut(part, "=")
		name := strings.TrimSpace(lhs)
		name = strings.Trim(name, "`")
		for _, pk := range pks {
			if strings.EqualFold(name, pk.name) {
				return true
			}
		}
	}
	return false
}

func pkForUpdateSQL(pks []pkColumn, table, whereSQL string) string {
	cols := make([]string, len(pks))
	for i, pk := range pks {
		cols[i] = mustQuote(pk.name)
	}
	sel := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), table)
	if whereSQL != "" {
		sel += " WHERE " + whereSQL
	}
	return sel + " FOR UPDATE"
}

func pkPredicate(pks []pkColumn, tuples [][]any) (string, []any) {
	args := make([]any, 0, len(pks)*len(tuples))
	if len(pks) == 1 {
		col := mustQuote(pks[0].name)
		if len(tuples) == 1 {
			return col + " = ?", append(args, tuples[0][0])
		}
		ph := make([]string, len(tuples))
		for i, tuple := range tuples {
			ph[i] = "?"
			args = append(args, tuple[0])
		}
		return col + " IN (" + strings.Join(ph, ",") + ")", args
	}
	cols := make([]string, len(pks))
	one := make([]string, len(pks))
	for i, pk := range pks {
		cols[i] = mustQuote(pk.name)
		one[i] = "?"
	}
	inner := "(" + strings.Join(one, ",") + ")"
	rowPH := make([]string, len(tuples))
	for i, tuple := range tuples {
		rowPH[i] = inner
		args = append(args, tuple...)
	}
	return "(" + strings.Join(cols, ",") + ") IN (" + strings.Join(rowPH, ",") + ")", args
}

func scanKeyTuples(rows *sql.Rows, n int) ([][]any, error) {
	var tuples [][]any
	for rows.Next() {
		holders := make([][]byte, n)
		dest := make([]any, n)
		for i := range holders {
			dest[i] = &holders[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		tuple := make([]any, n)
		for i, b := range holders {
			if b == nil {
				tuple[i] = nil
				continue
			}
			cp := make([]byte, len(b))
			copy(cp, b)
			tuple[i] = cp
		}
		tuples = append(tuples, tuple)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tuples, nil
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
	sel += " FOR UPDATE"
	rows, err := execQuery(ctx, tx, sel, params...)
	if err != nil {
		return scanErr(err)
	}
	buf, err := scanJSONArray(rows)
	rows.Close()
	if err != nil {
		return scanErr(err)
	}
	if _, err = execStmt(ctx, tx, stmt, params...); err != nil {
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
	res, err := execStmt(ctx, tx, query, params...)
	if err != nil {
		_ = tx.Rollback()
		return scanErr(err)
	}
	var inc int64
	if id, idErr := res.LastInsertId(); idErr == nil && id != 0 {
		inc, err = autoIncrementStep(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return scanErr(err)
		}
	}
	schema, table, cols, perr := parseInsert(query)
	var buf []byte
	if perr == nil {
		buf, err = a.batchImages(ctx, tx, schema, table, cols, params, res, inc)
	}
	if err != nil || buf == nil {
		buf, err = batchFallback(cols, params, res, inc)
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

func (a *Adapter) batchImages(ctx context.Context, tx *sql.Tx, schema, table string, cols []string, params []any, res sql.Result, inc int64) ([]byte, error) {
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
			v := params[row*len(cols)+idx]
			if _, ok := v.(adapters.DefaultValue); ok {
				// The key came from the column default; it cannot be selected back.
				return nil, fmt.Errorf("primary key omitted in a batch record")
			}
			args = append(args, v)
		}
	} else if pk.autoIncrement {
		id, err := res.LastInsertId()
		if err != nil || id == 0 {
			return nil, fmt.Errorf("missing last insert id")
		}
		for i := 0; i < n; i++ {
			args = append(args, id+int64(i)*inc)
		}
	} else {
		return nil, fmt.Errorf("primary key not in insert")
	}
	ph := make([]string, len(args))
	for i := range ph {
		ph[i] = "?"
	}
	q := fmt.Sprintf("SELECT * FROM %s.%s WHERE %s IN (%s)", mustQuote(schema), mustQuote(table), mustQuote(pk.name), strings.Join(ph, ","))
	rows, err := execQuery(ctx, tx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJSONArray(rows)
}

func batchFallback(cols []string, params []any, res sql.Result, inc int64) ([]byte, error) {
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
			v := params[row*len(cols)+i]
			if _, ok := v.(adapters.DefaultValue); ok {
				continue
			}
			obj[col] = v
		}
		if last != 0 {
			obj["last_insert_id"] = last + int64(row)*inc
		}
		out = append(out, obj)
	}
	if out == nil {
		out = []map[string]any{}
	}
	return json.Marshal(out)
}

func autoIncrementStep(ctx context.Context, tx *sql.Tx) (int64, error) {
	var step int64
	err := tx.QueryRowContext(ctx, "SELECT @@auto_increment_increment").Scan(&step)
	if err != nil {
		return 0, err
	}
	if step < 1 {
		return step, fmt.Errorf("auto_increment_increment %d", step)
	}
	return step, nil
}

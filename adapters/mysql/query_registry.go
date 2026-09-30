package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/internal/ident"
)

func (a *Adapter) queriesTable() (string, string) {
	schema, table := "public", "prest_queries"
	if a.cfg != nil {
		if a.cfg.QueriesConf.Schema != "" {
			schema = a.cfg.QueriesConf.Schema
		}
		if a.cfg.QueriesConf.Table != "" {
			table = a.cfg.QueriesConf.Table
		}
	}
	return schema, table
}

func (a *Adapter) qualifiedQueriesTable() (string, error) {
	schema, table := a.queriesTable()
	schemaQ, err := quoteIdent(schema)
	if err != nil {
		return "", err
	}
	tableQ, err := quoteIdent(table)
	if err != nil {
		return "", err
	}
	return schemaQ + "." + tableQ, nil
}

func (a *Adapter) ListQueries(ctx context.Context, databaseAlias, location string) ([]adapters.StoredQuery, error) {
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	qTable, err := a.qualifiedQueriesTable()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT id, database_alias, location, name, read_sql, write_sql, update_sql, delete_sql,
		description, created_by, CAST(created_at AS CHAR), CAST(updated_at AS CHAR)
		FROM %s WHERE 1=1`, qTable)
	args := make([]any, 0, 2)
	if databaseAlias != "" {
		query += " AND database_alias = ?"
		args = append(args, databaseAlias)
	}
	if location != "" {
		query += " AND location = ?"
		args = append(args, location)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list queries: %w", err)
	}
	defer rows.Close()
	return scanStoredQueries(rows)
}

func (a *Adapter) GetQuery(ctx context.Context, databaseAlias, location, name string) (adapters.StoredQuery, error) {
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return adapters.StoredQuery{}, err
	}
	qTable, err := a.qualifiedQueriesTable()
	if err != nil {
		return adapters.StoredQuery{}, err
	}
	query := fmt.Sprintf(`SELECT id, database_alias, location, name, read_sql, write_sql, update_sql, delete_sql,
		description, created_by, CAST(created_at AS CHAR), CAST(updated_at AS CHAR)
		FROM %s WHERE database_alias = ? AND location = ? AND name = ?`, qTable)
	var lastErr error
	for _, alias := range queryLookupAliases(databaseAlias) {
		rows, err := db.QueryContext(ctx, query, alias, location, name)
		if err != nil {
			return adapters.StoredQuery{}, fmt.Errorf("get query: %w", err)
		}
		list, scanErr := scanStoredQueries(rows)
		rows.Close()
		if scanErr != nil {
			return adapters.StoredQuery{}, scanErr
		}
		if len(list) > 0 {
			return list[0], nil
		}
		lastErr = sql.ErrNoRows
	}
	if lastErr != nil {
		return adapters.StoredQuery{}, fmt.Errorf("query not found: %w", lastErr)
	}
	return adapters.StoredQuery{}, fmt.Errorf("query not found")
}

func scanStoredQueries(rows *sql.Rows) ([]adapters.StoredQuery, error) {
	var out []adapters.StoredQuery
	for rows.Next() {
		var q adapters.StoredQuery
		var readSQL, writeSQL, updateSQL, deleteSQL, description, createdBy sql.NullString
		if err := rows.Scan(
			&q.ID, &q.DatabaseAlias, &q.Location, &q.Name,
			&readSQL, &writeSQL, &updateSQL, &deleteSQL,
			&description, &createdBy, &q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan query: %w", err)
		}
		q.ReadSQL = readSQL.String
		q.WriteSQL = writeSQL.String
		q.UpdateSQL = updateSQL.String
		q.DeleteSQL = deleteSQL.String
		q.Description = description.String
		q.CreatedBy = createdBy.String
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("list queries rows: %w", err)
	}
	return out, nil
}

func (a *Adapter) UpsertQuery(ctx context.Context, query adapters.StoredQuery) error {
	if err := validateQueryIdentity(query.DatabaseAlias, query.Location, query.Name); err != nil {
		return err
	}
	if query.ReadSQL == "" && query.WriteSQL == "" && query.UpdateSQL == "" && query.DeleteSQL == "" {
		return fmt.Errorf("at least one verb SQL column is required")
	}
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return err
	}
	qTable, err := a.qualifiedQueriesTable()
	if err != nil {
		return err
	}
	// Row alias (AS new) works on MySQL 8.0.19 through current innovation.
	// VALUES(col) is deprecated since 8.0.20 and is not used.
	stmt := fmt.Sprintf(`INSERT INTO %s
(database_alias, location, name, read_sql, write_sql, update_sql, delete_sql, description, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) AS new
ON DUPLICATE KEY UPDATE
  read_sql = new.read_sql,
  write_sql = new.write_sql,
  update_sql = new.update_sql,
  delete_sql = new.delete_sql,
  description = new.description,
  updated_at = CURRENT_TIMESTAMP(6)`, qTable)
	_, err = db.ExecContext(ctx, stmt,
		query.DatabaseAlias, query.Location, query.Name,
		nullString(query.ReadSQL), nullString(query.WriteSQL),
		nullString(query.UpdateSQL), nullString(query.DeleteSQL),
		nullString(query.Description), nullString(query.CreatedBy),
	)
	if err != nil {
		return fmt.Errorf("upsert query: %w", err)
	}
	return nil
}

func (a *Adapter) DeleteQuery(ctx context.Context, databaseAlias, location, name string) error {
	if err := validateQueryIdentity(databaseAlias, location, name); err != nil {
		return err
	}
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return err
	}
	qTable, err := a.qualifiedQueriesTable()
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE database_alias = ? AND location = ? AND name = ?`, qTable),
		databaseAlias, location, name)
	if err != nil {
		return fmt.Errorf("delete query: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete query rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("query not found")
	}
	return nil
}

func (a *Adapter) ImportFromFilesystem(ctx context.Context, queriesPath, policy string) (adapters.ImportReport, error) {
	scanned, err := scanFilesystemQueries(queriesPath)
	if err != nil {
		return adapters.ImportReport{}, err
	}
	var report adapters.ImportReport
	for _, sq := range scanned {
		existing, getErr := a.GetQuery(ctx, sq.DatabaseAlias, sq.Location, sq.Name)
		if getErr != nil {
			if err := a.UpsertQuery(ctx, sq); err != nil {
				return report, err
			}
			report.Inserted++
			continue
		}
		changed, conflict := diffQuery(existing, sq)
		if !changed {
			report.Skipped++
			continue
		}
		if conflict {
			switch policy {
			case config.QueriesImportPolicyError:
				return report, fmt.Errorf("import conflict for %s/%s", sq.Location, sq.Name)
			case config.QueriesImportPolicySkip:
				report.Skipped++
				continue
			}
		}
		merged := existing
		if sq.ReadSQL != "" {
			merged.ReadSQL = sq.ReadSQL
		}
		if sq.WriteSQL != "" {
			merged.WriteSQL = sq.WriteSQL
		}
		if sq.UpdateSQL != "" {
			merged.UpdateSQL = sq.UpdateSQL
		}
		if sq.DeleteSQL != "" {
			merged.DeleteSQL = sq.DeleteSQL
		}
		if err := a.UpsertQuery(ctx, merged); err != nil {
			return report, err
		}
		report.Updated++
	}
	return report, nil
}

func diffQuery(existing, incoming adapters.StoredQuery) (changed, conflict bool) {
	pairs := [][2]string{
		{existing.ReadSQL, incoming.ReadSQL},
		{existing.WriteSQL, incoming.WriteSQL},
		{existing.UpdateSQL, incoming.UpdateSQL},
		{existing.DeleteSQL, incoming.DeleteSQL},
	}
	for _, p := range pairs {
		if p[1] == "" || p[0] == p[1] {
			continue
		}
		changed = true
		if p[0] != "" {
			conflict = true
		}
	}
	return changed, conflict
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func validateQueryIdentity(databaseAlias, location, name string) error {
	if !ident.IsSafeSegment(location) {
		return fmt.Errorf("invalid location %q", location)
	}
	if !ident.IsSafeSegment(name) {
		return fmt.Errorf("invalid name %q", name)
	}
	if databaseAlias != "" && !ident.IsSafeSegment(databaseAlias) {
		return fmt.Errorf("invalid database %q", databaseAlias)
	}
	return nil
}

func scanFilesystemQueries(queriesPath string) ([]adapters.StoredQuery, error) {
	info, err := os.Stat(queriesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat queries path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("queries path is not a directory")
	}
	grouped := map[string]*adapters.StoredQuery{}
	err = filepath.WalkDir(queriesPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(queriesPath, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) < 2 {
			return nil
		}
		location := parts[0]
		fileName := parts[len(parts)-1]
		var matched string
		for suffix := range scriptSuffixColumns {
			if strings.HasSuffix(fileName, suffix) {
				matched = suffix
				break
			}
		}
		if matched == "" {
			return nil
		}
		name := strings.TrimSuffix(fileName, matched)
		if name == "" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		key := location + "\x00" + name
		sq, ok := grouped[key]
		if !ok {
			sq = &adapters.StoredQuery{Location: location, Name: name, CreatedBy: "filesystem-import"}
			grouped[key] = sq
		}
		switch scriptSuffixColumns[matched] {
		case "read_sql":
			sq.ReadSQL = string(body)
		case "write_sql":
			sq.WriteSQL = string(body)
		case "update_sql":
			sq.UpdateSQL = string(body)
		case "delete_sql":
			sq.DeleteSQL = string(body)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]adapters.StoredQuery, 0, len(grouped))
	for _, sq := range grouped {
		out = append(out, *sq)
	}
	return out, nil
}

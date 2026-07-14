package postgres

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

func (adapter *postgres) queriesTable() (schema, table string) {
	schema = adapter.cfg.QueriesConf.Schema
	table = adapter.cfg.QueriesConf.Table
	return schema, table
}

// queryLookupAliases returns database_alias values to try (imported scripts use "").
func queryLookupAliases(database string) []string {
	if database == "" {
		return []string{""}
	}
	return []string{database, ""}
}

func (adapter *postgres) qualifiedQueriesTable() (string, error) {
	schema, table := adapter.queriesTable()
	schemaQ, err := ident.Quote(schema)
	if err != nil {
		return "", err
	}
	tableQ, err := ident.Quote(table)
	if err != nil {
		return "", err
	}
	return schemaQ + "." + tableQ, nil
}

// ListQueries returns stored queries, optionally filtered.
func (adapter *postgres) ListQueries(ctx context.Context, databaseAlias, location string) ([]adapters.StoredQuery, error) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(
		`SELECT id, database_alias, location, name, read_sql, write_sql, update_sql, delete_sql,
		        description, created_by, created_at::text, updated_at::text
		   FROM %s WHERE 1=1`, qTable)
	args := make([]interface{}, 0, 2)
	argN := 1
	if databaseAlias != "" {
		query += fmt.Sprintf(" AND database_alias = $%d", argN)
		args = append(args, databaseAlias)
		argN++
	}
	if location != "" {
		query += fmt.Sprintf(" AND location = $%d", argN)
		args = append(args, location)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list queries: %w", err)
	}
	defer rows.Close()

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

// GetQuery returns one stored query.
func (adapter *postgres) GetQuery(ctx context.Context, databaseAlias, location, name string) (adapters.StoredQuery, error) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return adapters.StoredQuery{}, err
	}

	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return adapters.StoredQuery{}, err
	}
	query := fmt.Sprintf(
		`SELECT id, database_alias, location, name, read_sql, write_sql, update_sql, delete_sql,
		        description, created_by, created_at::text, updated_at::text
		   FROM %s
		  WHERE database_alias = $1 AND location = $2 AND name = $3`, qTable)

	var q adapters.StoredQuery
	var readSQL, writeSQL, updateSQL, deleteSQL, description, createdBy sql.NullString
	var lastErr error
	for _, alias := range queryLookupAliases(databaseAlias) {
		err = db.QueryRowContext(ctx, query, alias, location, name).Scan(
			&q.ID, &q.DatabaseAlias, &q.Location, &q.Name,
			&readSQL, &writeSQL, &updateSQL, &deleteSQL,
			&description, &createdBy, &q.CreatedAt, &q.UpdatedAt,
		)
		if err == nil {
			lastErr = nil
			break
		}
		if err != sql.ErrNoRows {
			return adapters.StoredQuery{}, fmt.Errorf("get query: %w", err)
		}
		lastErr = err
	}
	if lastErr != nil {
		return adapters.StoredQuery{}, fmt.Errorf("query not found: %w", lastErr)
	}
	q.ReadSQL = readSQL.String
	q.WriteSQL = writeSQL.String
	q.UpdateSQL = updateSQL.String
	q.DeleteSQL = deleteSQL.String
	q.Description = description.String
	q.CreatedBy = createdBy.String
	return q, nil
}

// UpsertQuery inserts or updates a stored query.
func (adapter *postgres) UpsertQuery(ctx context.Context, query adapters.StoredQuery) error {
	if err := validateQueryIdentity(query.DatabaseAlias, query.Location, query.Name); err != nil {
		return err
	}
	if !hasAnyVerbSQL(query) {
		return fmt.Errorf("at least one verb SQL column is required")
	}

	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return err
	}

	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return err
	}
	sqlStmt := fmt.Sprintf(`
INSERT INTO %s (database_alias, location, name, read_sql, write_sql, update_sql, delete_sql, description, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (database_alias, location, name) DO UPDATE SET
  read_sql = EXCLUDED.read_sql,
  write_sql = EXCLUDED.write_sql,
  update_sql = EXCLUDED.update_sql,
  delete_sql = EXCLUDED.delete_sql,
  description = EXCLUDED.description,
  updated_at = now()`, qTable)

	_, err = db.ExecContext(ctx, sqlStmt,
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

// DeleteQuery removes a stored query.
func (adapter *postgres) DeleteQuery(ctx context.Context, databaseAlias, location, name string) error {
	if err := validateQueryIdentity(databaseAlias, location, name); err != nil {
		return err
	}
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return err
	}
	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return err
	}
	res, err := db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE database_alias = $1 AND location = $2 AND name = $3`, qTable),
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

// ImportFromFilesystem scans queries.location and syncs into prest_queries.
func (adapter *postgres) ImportFromFilesystem(ctx context.Context, queriesPath, policy string) (adapters.ImportReport, error) {
	scanned, err := scanFilesystemQueries(queriesPath)
	if err != nil {
		return adapters.ImportReport{}, err
	}

	report := adapters.ImportReport{}
	for _, sq := range scanned {
		existing, getErr := adapter.GetQuery(ctx, sq.DatabaseAlias, sq.Location, sq.Name)
		if getErr != nil {
			if err := adapter.UpsertQuery(ctx, sq); err != nil {
				return report, err
			}
			report.Inserted++
			continue
		}

		changed, conflict, err := diffStoredQuery(existing, sq)
		if err != nil {
			return report, err
		}
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

		merged := mergeStoredQuery(existing, sq)
		if err := adapter.patchQuery(ctx, merged, diffColumns(existing, sq)); err != nil {
			return report, err
		}
		report.Updated++
	}
	return report, nil
}

func (adapter *postgres) patchQuery(ctx context.Context, query adapters.StoredQuery, cols []string) error {
	if len(cols) == 0 {
		return nil
	}
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return err
	}

	sets := make([]string, 0, len(cols)+1)
	args := make([]interface{}, 0, len(cols)+3)
	argN := 1
	for _, col := range cols {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, argN))
		argN++
		switch col {
		case "read_sql":
			args = append(args, nullString(query.ReadSQL))
		case "write_sql":
			args = append(args, nullString(query.WriteSQL))
		case "update_sql":
			args = append(args, nullString(query.UpdateSQL))
		case "delete_sql":
			args = append(args, nullString(query.DeleteSQL))
		}
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, query.DatabaseAlias, query.Location, query.Name)

	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return err
	}
	stmt := fmt.Sprintf(
		`UPDATE %s SET %s WHERE database_alias = $%d AND location = $%d AND name = $%d`,
		qTable, strings.Join(sets, ", "), argN, argN+1, argN+2)
	_, err = db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("patch query: %w", err)
	}
	return nil
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func hasAnyVerbSQL(q adapters.StoredQuery) bool {
	return q.ReadSQL != "" || q.WriteSQL != "" || q.UpdateSQL != "" || q.DeleteSQL != ""
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

type filesystemScript struct {
	adapters.StoredQuery
	Files map[string]string // column -> content
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

	grouped := make(map[string]*filesystemScript)
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

		var matchedSuffix string
		for suffix := range scriptSuffixColumns {
			if strings.HasSuffix(fileName, suffix) {
				matchedSuffix = suffix
				break
			}
		}
		if matchedSuffix == "" {
			return nil
		}
		name := strings.TrimSuffix(fileName, matchedSuffix)
		if name == "" {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		col := scriptSuffixColumns[matchedSuffix]
		key := location + "\x00" + name
		fs, ok := grouped[key]
		if !ok {
			fs = &filesystemScript{
				StoredQuery: adapters.StoredQuery{
					Location:  location,
					Name:      name,
					CreatedBy: "filesystem-import",
				},
				Files: make(map[string]string),
			}
			grouped[key] = fs
		}
		fs.Files[col] = string(body)
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]adapters.StoredQuery, 0, len(grouped))
	for _, fs := range grouped {
		for col, content := range fs.Files {
			switch col {
			case "read_sql":
				fs.ReadSQL = content
			case "write_sql":
				fs.WriteSQL = content
			case "update_sql":
				fs.UpdateSQL = content
			case "delete_sql":
				fs.DeleteSQL = content
			}
		}
		out = append(out, fs.StoredQuery)
	}
	return out, nil
}

func diffStoredQuery(existing, incoming adapters.StoredQuery) (changed bool, conflict bool, err error) {
	cols := []struct {
		col string
		old string
		new string
	}{
		{"read_sql", existing.ReadSQL, incoming.ReadSQL},
		{"write_sql", existing.WriteSQL, incoming.WriteSQL},
		{"update_sql", existing.UpdateSQL, incoming.UpdateSQL},
		{"delete_sql", existing.DeleteSQL, incoming.DeleteSQL},
	}
	for _, c := range cols {
		if c.new == "" {
			continue
		}
		if c.old == c.new {
			continue
		}
		changed = true
		if c.old != "" && c.old != c.new {
			conflict = true
		}
	}
	return changed, conflict, nil
}

func diffColumns(existing, incoming adapters.StoredQuery) []string {
	var cols []string
	pairs := []struct {
		col string
		old string
		new string
	}{
		{"read_sql", existing.ReadSQL, incoming.ReadSQL},
		{"write_sql", existing.WriteSQL, incoming.WriteSQL},
		{"update_sql", existing.UpdateSQL, incoming.UpdateSQL},
		{"delete_sql", existing.DeleteSQL, incoming.DeleteSQL},
	}
	for _, p := range pairs {
		if p.new == "" || p.old == p.new {
			continue
		}
		cols = append(cols, p.col)
	}
	return cols
}

func mergeStoredQuery(existing, incoming adapters.StoredQuery) adapters.StoredQuery {
	out := existing
	if incoming.ReadSQL != "" {
		out.ReadSQL = incoming.ReadSQL
	}
	if incoming.WriteSQL != "" {
		out.WriteSQL = incoming.WriteSQL
	}
	if incoming.UpdateSQL != "" {
		out.UpdateSQL = incoming.UpdateSQL
	}
	if incoming.DeleteSQL != "" {
		out.DeleteSQL = incoming.DeleteSQL
	}
	return out
}

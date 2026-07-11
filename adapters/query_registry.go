package adapters

import "context"

// StoredQuery is a row from prest_queries.
type StoredQuery struct {
	ID            int64  `json:"id,omitempty"`
	DatabaseAlias string `json:"database"`
	Location      string `json:"location"`
	Name          string `json:"name"`
	ReadSQL       string `json:"read_sql,omitempty"`
	WriteSQL      string `json:"write_sql,omitempty"`
	UpdateSQL     string `json:"update_sql,omitempty"`
	DeleteSQL     string `json:"delete_sql,omitempty"`
	Description   string `json:"description,omitempty"`
	CreatedBy     string `json:"created_by,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// ImportReport summarizes a filesystem import run.
type ImportReport struct {
	Inserted int
	Updated  int
	Skipped  int
}

// QueryRegistry manages prest_queries rows.
type QueryRegistry interface {
	ListQueries(ctx context.Context, databaseAlias, location string) ([]StoredQuery, error)
	GetQuery(ctx context.Context, databaseAlias, location, name string) (StoredQuery, error)
	UpsertQuery(ctx context.Context, query StoredQuery) error
	DeleteQuery(ctx context.Context, databaseAlias, location, name string) error
	ImportFromFilesystem(ctx context.Context, queriesPath, policy string) (ImportReport, error)
}

// ScriptPermissionsChecker validates custom query execution access.
type ScriptPermissionsChecker interface {
	ScriptPermissions(databaseAlias, location, name, op, userName string) bool
}

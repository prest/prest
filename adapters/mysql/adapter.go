package mysql

import (
	"context"
	"database/sql"
	"slices"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
)

// Adapter is the MySQL 8.0+ dialect adapter (8.0.19 or newer; see integration/mysql/DIFFERENCES.md).
type Adapter struct {
	cfg     *config.Prest
	conn    *connection.Manager
	pkMu    sync.Mutex
	pkCache map[string][]pkColumn
}

var (
	_ adapters.Adapter                  = (*Adapter)(nil)
	_ adapters.DatabaseConnector        = (*Adapter)(nil)
	_ adapters.DatabaseAccessor         = (*Adapter)(nil)
	_ adapters.DatabasePinger           = (*Adapter)(nil)
	_ adapters.QueryRegistry            = (*Adapter)(nil)
	_ adapters.ScriptPermissionsChecker = (*Adapter)(nil)
	_ adapters.SystemTableEnsurer       = (*Adapter)(nil)
	_ adapters.DatabaseSchemaScoper     = (*Adapter)(nil)
)

// New creates a MySQL adapter without connecting.
func New(cfg *config.Prest) adapters.Adapter {
	return &Adapter{
		cfg:     cfg,
		conn:    connection.NewManager(cfg),
		pkCache: map[string][]pkColumn{},
	}
}

// Connect opens the pool and pings it.
func (a *Adapter) Connect() error {
	if a.conn.GetDatabase() == "" && a.cfg != nil {
		a.conn.SetDatabase(a.cfg.PGDatabase)
	}
	db, err := a.conn.Get()
	if err != nil {
		return err
	}
	return db.Ping()
}

// DB returns the pooled connection.
func (a *Adapter) DB() (*sqlx.DB, error) {
	return a.conn.Get()
}

// Ping verifies the pooled connection.
func (a *Adapter) Ping(ctx context.Context) error {
	db, err := a.conn.Get()
	if err != nil {
		return wrapDriver(err)
	}
	return wrapDriver(db.PingContext(ctx))
}

// PingAll pings connections this manager already owns.
func (a *Adapter) PingAll(ctx context.Context) error {
	return a.Ping(ctx)
}

// SetDatabase records the active alias. It does not issue USE.
func (a *Adapter) SetDatabase(name string) { a.conn.SetDatabase(name) }

// GetDatabase returns the active alias.
func (a *Adapter) GetDatabase() string { return a.conn.GetDatabase() }

// servedDatabases lists the databases reachable without a registry:
// pg.database plus pg.allowed_databases, empties and duplicates dropped.
// nil in registry mode, where the alias is the source of truth; empty (not
// nil) for a nil config, so catalog filters match nothing.
func (a *Adapter) servedDatabases() []string {
	if a.cfg == nil {
		return []string{}
	}
	if a.cfg.HasDatabaseRegistry() {
		return nil
	}
	return dedupeNonEmpty(append([]string{a.cfg.PGDatabase}, a.cfg.AllowedDatabases...))
}

// dedupeNonEmpty keeps the first occurrence of each non-empty name, in order.
func dedupeNonEmpty(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Aliases returns the names this adapter serves: the registry alias, or
// without a registry pg.database plus pg.allowed_databases.
func (a *Adapter) Aliases() []string {
	if a.cfg == nil || !a.cfg.HasDatabaseRegistry() {
		if served := a.servedDatabases(); len(served) > 0 {
			return served
		}
		return nil
	}
	if name := a.GetDatabase(); name != "" {
		return []string{name}
	}
	if a.cfg.PGDatabase != "" {
		return []string{a.cfg.PGDatabase}
	}
	return nil
}

// IsRegistered reports whether alias selects this adapter's connection.
// Without a registry only the served databases are accepted, matching the
// Postgres adapter.
func (a *Adapter) IsRegistered(alias string) bool {
	if a.cfg == nil {
		return false
	}
	if !a.cfg.HasDatabaseRegistry() {
		return slices.Contains(a.servedDatabases(), alias)
	}
	return a.conn.Owns(alias)
}

// PhysicalName is the MySQL database alias resolves to. Without a registry a
// served name is itself a database; otherwise it is the connection's
// pg.database.
func (a *Adapter) PhysicalName(alias string) string {
	if a.cfg == nil {
		return alias
	}
	if alias != "" && !a.cfg.HasDatabaseRegistry() && slices.Contains(a.servedDatabases(), alias) {
		return alias
	}
	if a.cfg.PGDatabase != "" {
		return a.cfg.PGDatabase
	}
	return alias
}

// AllowsDatabaseSchema pins {schema} to the routed database, since MySQL schemas are
// databases: with a registry, the entry's physical database; without one,
// {database} itself. One served name can never reach another's database.
func (a *Adapter) AllowsDatabaseSchema(database, schema string) bool {
	if a.cfg == nil || !a.cfg.HasDatabaseRegistry() {
		return schema == database
	}
	return schema == a.PhysicalName("")
}

// GetTransaction starts a transaction on the pooled connection.
func (a *Adapter) GetTransaction() (*sql.Tx, error) {
	db, err := a.conn.Get()
	if err != nil {
		return nil, err
	}
	return db.Begin()
}

// GetTransactionCtx starts a repeatable-read transaction.
func (a *Adapter) GetTransactionCtx(ctx context.Context) (*sql.Tx, error) {
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	return db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
}

// dbFromCtx returns the pool for the request's alias. Connection failures are
// wrapped so callers never surface dial text.
func (a *Adapter) dbFromCtx(ctx context.Context) (*sqlx.DB, error) {
	if ctx != nil {
		if name, ok := ctx.Value(pctx.DBNameKey).(string); ok && name != "" {
			db, err := a.conn.GetOwned(name)
			return db, wrapDriver(err)
		}
	}
	db, err := a.conn.Get()
	return db, wrapDriver(err)
}

// Connect initializes a MySQL adapter connection pool.
func Connect(ad adapters.Adapter) error {
	c, ok := ad.(adapters.DatabaseConnector)
	if !ok {
		return errInvalidIdentifier
	}
	return c.Connect()
}

// SetDBConnectForTest replaces sqlx.Connect for unit tests outside this package.
// Callers must not use t.Parallel().
func SetDBConnectForTest(fn func(driverName, dataSourceName string) (*sqlx.DB, error)) func() {
	return connection.SetDBConnectForTest(fn)
}

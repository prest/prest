package mysql

import (
	"context"
	"database/sql"
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
		return err
	}
	return db.PingContext(ctx)
}

// PingAll pings connections this manager already owns.
func (a *Adapter) PingAll(ctx context.Context) error {
	return a.Ping(ctx)
}

// SetDatabase records the active alias. It does not issue USE.
func (a *Adapter) SetDatabase(name string) { a.conn.SetDatabase(name) }

// GetDatabase returns the active alias.
func (a *Adapter) GetDatabase() string { return a.conn.GetDatabase() }

// Aliases returns the alias this adapter serves.
func (a *Adapter) Aliases() []string {
	if name := a.GetDatabase(); name != "" {
		return []string{name}
	}
	if a.cfg != nil && a.cfg.PGDatabase != "" {
		return []string{a.cfg.PGDatabase}
	}
	return nil
}

// IsRegistered reports whether alias selects this adapter's connection.
// Without a registry every name is accepted, matching the Postgres adapter.
func (a *Adapter) IsRegistered(alias string) bool {
	if a.cfg == nil || !a.cfg.HasDatabaseRegistry() {
		return true
	}
	return a.conn.Owns(alias)
}

// PhysicalName is the MySQL database this connection was opened against.
func (a *Adapter) PhysicalName(alias string) string {
	if a.cfg == nil {
		return alias
	}
	if a.cfg.PGDatabase != "" {
		return a.cfg.PGDatabase
	}
	return alias
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

func (a *Adapter) dbFromCtx(ctx context.Context) (*sqlx.DB, error) {
	if ctx != nil {
		if name, ok := ctx.Value(pctx.DBNameKey).(string); ok && name != "" {
			return a.conn.GetOwned(name)
		}
	}
	return a.conn.Get()
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

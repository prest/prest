package postgres

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/prest/prest/v2/internal/adapters/postgres/connection"
	pctx "github.com/prest/prest/v2/internal/contextkeys"
	"github.com/prest/prest/v2/pkg/adapters"
	"github.com/prest/prest/v2/pkg/config"

	"github.com/jmoiron/sqlx"
)

// postgres adapter
type postgres struct {
	cfg     *config.Prest
	conn    *connection.Manager
	stmts   *Stmt
	stmtsMu sync.Mutex
}

var (
	_ adapters.Adapter                  = (*postgres)(nil)
	_ adapters.DatabaseConnector        = (*postgres)(nil)
	_ adapters.DatabaseAccessor         = (*postgres)(nil)
	_ adapters.DatabasePinger           = (*postgres)(nil)
	_ adapters.QueryRegistry            = (*postgres)(nil)
	_ adapters.ScriptPermissionsChecker = (*postgres)(nil)
)

const (
	pageNumberKey = "_page"
	pageSizeKey   = "_page_size"

	defaultPageSize   = 10
	defaultPageNumber = 1
)

var removeOperatorRegex *regexp.Regexp
var insertTableNameQuotesRegex *regexp.Regexp
var insertTableNameRegex *regexp.Regexp

func init() {
	removeOperatorRegex = regexp.MustCompile(`\$[a-z]+.`)
	insertTableNameRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.|-]*\.)*([\w|-]+)\s*\(`)
	insertTableNameQuotesRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.|"|-]*\.)*"([\w|-]+)"\s*\(`)
}

// New creates a Postgres adapter without connecting.
func New(cfg *config.Prest) adapters.Adapter {
	return &postgres{
		cfg:  cfg,
		conn: connection.NewManager(cfg, otelManagerOptions(cfg)...),
	}
}

// Connect initializes the database connection pool and verifies connectivity.
func (p *postgres) Connect() error {
	if p.conn.GetDatabase() == "" {
		p.conn.SetDatabase(p.cfg.PGDatabase)
	}
	db, err := p.conn.Get()
	if err != nil {
		return err
	}
	return db.Ping()
}

// DB returns the current database connection.
func (p *postgres) DB() (*sqlx.DB, error) {
	return p.conn.Get()
}

// Ping verifies the default database connection is alive.
func (p *postgres) Ping(ctx context.Context) error {
	db, err := p.conn.Get()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

// PingAll verifies the default and all registered database connections are alive.
func (p *postgres) PingAll(ctx context.Context) error {
	if err := p.Ping(ctx); err != nil {
		return err
	}
	if !p.cfg.HasDatabaseRegistry() {
		return nil
	}
	for _, dbConf := range p.cfg.Databases {
		conn, err := p.conn.AddDatabaseToPool(dbConf.Alias)
		if err != nil {
			return err
		}
		if err := conn.PingContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Aliases returns configured database aliases or the default database name.
func (p *postgres) Aliases() []string {
	if p.cfg.HasDatabaseRegistry() {
		aliases := make([]string, 0, len(p.cfg.Databases))
		for _, dbConf := range p.cfg.Databases {
			aliases = append(aliases, dbConf.Alias)
		}
		return aliases
	}
	if p.cfg.PGDatabase == "" {
		return nil
	}
	return []string{p.cfg.PGDatabase}
}

// IsRegistered reports whether alias is a configured database registry entry.
func (p *postgres) IsRegistered(alias string) bool {
	if !p.cfg.HasDatabaseRegistry() {
		return true
	}
	_, ok := p.cfg.ProfileByAlias(alias)
	return ok
}

// PhysicalName resolves a registry alias to its physical database name.
func (p *postgres) PhysicalName(alias string) string {
	if conf, ok := p.cfg.ProfileByAlias(alias); ok && conf.Database != "" {
		return conf.Database
	}
	if alias == "" {
		return p.cfg.PGDatabase
	}
	return alias
}

// GetDatabase returns the current DB name
func (adapter *postgres) GetDatabase() string {
	return adapter.conn.GetDatabase()
}

// dbFromCtx tries to get the DB from context adding it to the pool if not
// present, unless DB name is unset in the context - it will then fallback to
// the current DB has been set via `SetDatabase(...)`
func (adapter *postgres) dbFromCtx(ctx context.Context) (db *sqlx.DB, err error) {
	dbName, ok := ctx.Value(pctx.DBNameKey).(string)
	if ok {
		DB, err := adapter.conn.GetFromPool(dbName)
		if err == nil {
			return DB, err
		}
		return adapter.conn.AddDatabaseToPool(dbName)
	}
	return adapter.conn.Get()
}

// chkInvalidIdentifier return true if identifier is invalid
func chkInvalidIdentifier(identifier ...string) bool {
	for _, ival := range identifier {
		if ival == "" || unicode.IsDigit([]rune(ival)[0]) {
			return true
		}

		ivalSplit := strings.Split(ival, ".")
		if len(ivalSplit) == 2 && len(ivalSplit[len(ivalSplit)-1]) > 63 {
			return true
		}

		if !strings.Contains(ival, ".") && len(ival) > 63 {
			return true
		}

		count := 0
		for _, v := range ival {
			if !unicode.IsLetter(v) &&
				!unicode.IsDigit(v) &&
				v != '(' &&
				v != ')' &&
				v != '_' &&
				v != '.' &&
				v != '-' &&
				v != '*' &&
				v != '[' &&
				v != ']' &&
				v != '"' {
				return true
			}
			if unicode.Is(unicode.Quotation_Mark, v) {
				count++
			}
		}
		if count%2 != 0 {
			return true
		}
	}
	return false
}

package app

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"
	"github.com/prest/prest/v2/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// TestScopedConfig_ZeroPoolKeepsGlobal: an entry without pool limits keeps
// the global ones instead of passing 0 (unlimited) to database/sql.
func TestScopedConfig_ZeroPoolKeepsGlobal(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2, PGDatabase: "root"}
	got := scopedConfig(cfg, &config.DatabaseConf{Alias: "a", Database: "a_db", Host: "h", Port: 6543})
	require.Equal(t, 10, got.PGMaxOpenConn)
	require.Equal(t, 2, got.PGMaxIdleConn)
	require.Equal(t, "a_db", got.PGDatabase)
	require.Equal(t, "h", got.PGHost)
	require.Equal(t, 6543, got.PGPort)
	require.Equal(t, config.EnginePostgres, got.Engine)
	require.Equal(t, "root", cfg.PGDatabase, "the root config must not be modified")
}

// TestScopedConfig_ExplicitPoolAndURL: explicit limits win, and a mysql URL is
// only copied for a mysql entry when it carries the mysql scheme.
func TestScopedConfig_ExplicitPoolAndURL(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2}
	got := scopedConfig(cfg, &config.DatabaseConf{
		Alias: "a", URL: "postgres://u@h/a", MaxOpenConn: 25, MaxIdleConn: 5,
	})
	require.Equal(t, 25, got.PGMaxOpenConn)
	require.Equal(t, 5, got.PGMaxIdleConn)
	require.Equal(t, "postgres://u@h/a", got.PGURL)

	my := scopedConfig(cfg, &config.DatabaseConf{
		Alias: "shop", Engine: config.EngineMySQL, Database: "shop", URL: "postgres://ignored",
	})
	require.Equal(t, config.EngineMySQL, my.Engine)
	require.Empty(t, my.PGURL)
	require.Equal(t, "shop", my.AuthSchema)
}

// TestEnsureSchemaMigrated_SkipsUnreachable: auth migration runs only on the
// reachable alias; the unreachable one is skipped instead of aborting startup.
func TestEnsureSchemaMigrated_SkipsUnreachable(t *testing.T) {
	t.Parallel()

	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sqlxDB := sqlx.NewDb(db, "postgres")
	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// A nil *sqlx.DB on "down" would make the migration fail if it were tried.
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("down", &migrateDBAdapter{}))
	require.NoError(t, registry.Register("live", &migrateDBAdapter{db: sqlxDB}))

	cfg := &config.Prest{
		Databases:            []config.DatabaseConf{{Alias: "down"}, {Alias: "live"}},
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
		AuthSchema:           "public",
		AuthTable:            "prest_users",
	}
	require.NoError(t, ensureSchemaMigrated(cfg, registry, map[string]struct{}{"down": {}}))
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

// TestDefaultAdapter covers the selection order: first configured alias that
// was reachable, else the first configured alias, else nil.
func TestDefaultAdapter(t *testing.T) {
	t.Parallel()

	a, b := &migrateDBAdapter{}, &migrateDBAdapter{}
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("a", a))
	require.NoError(t, registry.Register("b", b))
	cfg := &config.Prest{Databases: []config.DatabaseConf{{Alias: "a"}, {Alias: "b"}}}

	require.Same(t, a, defaultAdapter(cfg, registry, nil))
	require.Same(t, b, defaultAdapter(cfg, registry, map[string]struct{}{"a": {}}))
	require.Same(t, a, defaultAdapter(cfg, registry, map[string]struct{}{"a": {}, "b": {}}))
	require.Nil(t, defaultAdapter(&config.Prest{}, registry, nil))
}

// TestNew_RoutesRequestToMatchingAdapter: through the full app handler,
// /a/public/t and /b/public/t each reach their own adapter. Regression guard
// for the adapter selector running after route matching.
func TestNew_RoutesRequestToMatchingAdapter(t *testing.T) {
	byAlias := map[string]*mock.Mock{"a": mock.New(t), "b": mock.New(t)}
	byAlias["a"].AddItem([]byte(`[{"from":"a"}]`), nil, false)
	byAlias["b"].AddItem([]byte(`[{"from":"b"}]`), nil, false)

	orig := connectAdapter
	connectAdapter = func(_ *config.Prest, dbConf *config.DatabaseConf) (adapters.Adapter, error) {
		return byAlias[dbConf.Alias], nil
	}
	t.Cleanup(func() { connectAdapter = orig })

	cfg := &config.Prest{Databases: []config.DatabaseConf{{Alias: "a"}, {Alias: "b"}}}
	got, err := New(cfg)
	require.NoError(t, err)

	for _, alias := range []string{"b", "a"} {
		rec := httptest.NewRecorder()
		got.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+alias+"/public/t", nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.JSONEq(t, `[{"from":"`+alias+`"}]`, rec.Body.String())
		require.Empty(t, byAlias[alias].Items, "adapter %s must serve its own request", alias)
	}
}

// TestLogServedDatabases: with pg.single=false and no registry, start-up logs
// exactly which database names are reachable; other modes log nothing.
func TestLogServedDatabases(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	logServedDatabases(&config.Prest{PGDatabase: "main", AllowedDatabases: []string{"extra"}})
	require.Contains(t, buf.String(), "pg.single=false")
	require.Contains(t, buf.String(), "main")
	require.Contains(t, buf.String(), "extra")

	buf.Reset()
	logServedDatabases(&config.Prest{PGDatabase: "main", SingleDB: true})
	logServedDatabases(&config.Prest{PGDatabase: "main", Databases: []config.DatabaseConf{{Alias: "a"}}})
	require.Empty(t, buf.String())
}

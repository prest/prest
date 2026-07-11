package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"
	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestPostgresDB_ErrAdapterNotPostgres(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter: &mock.Mock{},
	}
	_, err := app.PostgresDB(cfg)
	require.ErrorIs(t, err, app.ErrAdapterNotPostgres)
}

func TestPostgresDB_EnsureAdapterConnects(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGHost:     "invalid-host",
		PGPort:     1,
		PGUser:     "x",
		PGDatabase: "x",
		PGSSLMode:  "disable",
	}
	_, err := app.PostgresDB(cfg)
	require.Error(t, err)
}

func TestPostgresDB_GenericDBError(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("database unavailable")
	cfg := &config.Prest{
		Adapter: &failingDBAdapter{Mock: mock.New(t), err: dbErr},
	}
	_, err := app.PostgresDB(cfg)
	require.ErrorIs(t, err, dbErr)
}

func TestEnsureAdapter_AlreadyConfigured(t *testing.T) {
	t.Parallel()

	adapter := mock.New(t)
	cfg := &config.Prest{Adapter: adapter}
	require.NoError(t, app.EnsureAdapter(cfg))
	require.Same(t, adapter, cfg.Adapter)
}

func TestEnsureAdapter_ConnectError(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGHost:     "invalid-host",
		PGPort:     1,
		PGUser:     "x",
		PGDatabase: "x",
		PGSSLMode:  "disable",
	}
	err := app.EnsureAdapter(cfg)
	require.Error(t, err)
	require.Nil(t, cfg.Adapter)
}

func TestPostgresDB_Success(t *testing.T) {
	t.Parallel()

	adapter, _ := newDBAdapter(t)
	cfg := &config.Prest{Adapter: adapter}

	db, err := app.PostgresDB(cfg)
	require.NoError(t, err)
	require.NotNil(t, db)
}

func TestNew_ErrAdapterNotQueryRegistry(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:     mock.New(t),
		QueriesPath: "/queries",
		QueriesConf: config.QueriesConf{
			Storage:          config.QueriesStorageDatabase,
			ImportOnStartup:  true,
			MigrateOnStartup: false,
		},
	}
	_, err := app.New(cfg)
	require.ErrorIs(t, err, app.ErrAdapterNotQueryRegistry)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *config.Prest
		wantErr bool
	}{
		{
			name: "success with existing adapter",
			setup: func(t *testing.T) *config.Prest {
				return &config.Prest{
					Adapter:    mock.New(t),
					PGDatabase: "prest",
				}
			},
		},
		{
			name: "connect error when adapter is nil",
			setup: func(t *testing.T) *config.Prest {
				return &config.Prest{
					PGHost:     "invalid-host",
					PGPort:     1,
					PGUser:     "x",
					PGDatabase: "x",
					PGSSLMode:  "disable",
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.setup(t)
			adapterBefore := cfg.Adapter

			got, err := app.New(cfg)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotNil(t, got.Handler)
			require.Same(t, cfg, got.Config)
			if adapterBefore != nil {
				require.Same(t, adapterBefore, cfg.Adapter)
			}
		})
	}
}

func TestNew_SchemaMigration_AuthOnly(t *testing.T) {
	t.Parallel()

	adapter, sqlMock := newDBAdapter(t)
	expectAuthTableMigration(sqlMock)

	cfg := &config.Prest{
		Adapter:              adapter,
		PGDatabase:           "prest",
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
		AuthSchema:           "public",
		AuthTable:            "prest_users",
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestNew_SchemaMigration_QueriesOnly(t *testing.T) {
	t.Parallel()

	adapter, sqlMock := newDBAdapter(t)
	expectQueriesTableMigration(sqlMock)

	cfg := &config.Prest{
		Adapter:    adapter,
		PGDatabase: "prest",
		QueriesConf: config.QueriesConf{
			Storage:          config.QueriesStorageDatabase,
			MigrateOnStartup: true,
			Schema:           "public",
			Table:            "prest_queries",
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestNew_SchemaMigration_Both(t *testing.T) {
	t.Parallel()

	adapter, sqlMock := newDBAdapter(t)
	expectAuthTableMigration(sqlMock)
	expectQueriesTableMigration(sqlMock)

	cfg := &config.Prest{
		Adapter:              adapter,
		PGDatabase:           "prest",
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
		AuthSchema:           "public",
		AuthTable:            "prest_users",
		QueriesConf: config.QueriesConf{
			Storage:          config.QueriesStorageDatabase,
			MigrateOnStartup: true,
			Schema:           "public",
			Table:            "prest_queries",
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestNew_SchemaMigration_PostgresDBError(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:              mock.New(t),
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
	}
	_, err := app.New(cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, app.ErrAdapterNotPostgres)
	require.Contains(t, err.Error(), "acquire database connection for startup migration")
}

func TestNew_SchemaMigration_AuthTableError(t *testing.T) {
	t.Parallel()

	adapter, sqlMock := newDBAdapter(t)
	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_users"`).
		WillReturnError(errors.New("auth migration failed"))

	cfg := &config.Prest{
		Adapter:              adapter,
		PGDatabase:           "prest",
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
		AuthSchema:           "public",
		AuthTable:            "prest_users",
	}
	_, err := app.New(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate auth table public.prest_users")
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestNew_SchemaMigration_QueriesTableError(t *testing.T) {
	t.Parallel()

	adapter, sqlMock := newDBAdapter(t)
	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_queries"`).
		WillReturnError(errors.New("queries migration failed"))

	cfg := &config.Prest{
		Adapter:    adapter,
		PGDatabase: "prest",
		QueriesConf: config.QueriesConf{
			Storage:          config.QueriesStorageDatabase,
			MigrateOnStartup: true,
			Schema:           "public",
			Table:            "prest_queries",
		},
	}
	_, err := app.New(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "migrate queries table public.prest_queries")
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

func TestNew_QueriesImport_SkipWhenFilesystemStorage(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:     mock.New(t),
		QueriesPath: "/queries",
		PGDatabase:  "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageFilesystem,
			ImportOnStartup: true,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestNew_QueriesImport_SkipWhenImportDisabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:     mock.New(t),
		QueriesPath: "/queries",
		PGDatabase:  "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageDatabase,
			ImportOnStartup: false,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestNew_QueriesImport_SkipWhenPathEmpty(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:    mock.New(t),
		PGDatabase: "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestNew_QueriesImport_Success(t *testing.T) {
	t.Parallel()

	adapter, _ := newDBAdapter(t)
	registry := &queryRegistryAdapter{
		dbAdapter: adapter,
		importFn: func(_ context.Context, queriesPath, _ string) (adapters.ImportReport, error) {
			require.Equal(t, "/queries", queriesPath)
			return adapters.ImportReport{Inserted: 2, Updated: 1, Skipped: 3}, nil
		},
	}

	cfg := &config.Prest{
		Adapter:     registry,
		QueriesPath: "/queries",
		PGDatabase:  "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestNew_QueriesImport_Error(t *testing.T) {
	t.Parallel()

	importErr := errors.New("import failed")
	adapter, _ := newDBAdapter(t)
	registry := &queryRegistryAdapter{
		dbAdapter: adapter,
		importFn: func(context.Context, string, string) (adapters.ImportReport, error) {
			return adapters.ImportReport{}, importErr
		},
	}

	cfg := &config.Prest{
		Adapter:     registry,
		QueriesPath: "/queries",
		PGDatabase:  "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	_, err := app.New(cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, importErr)
	require.Contains(t, err.Error(), "import query scripts from /queries")
}

func TestNew_QueriesImport_EnvLocation(t *testing.T) {
	t.Setenv("PREST_QUERIES_LOCATION", "/env/queries")

	adapter, _ := newDBAdapter(t)
	registry := &queryRegistryAdapter{
		dbAdapter: adapter,
		importFn: func(_ context.Context, queriesPath, _ string) (adapters.ImportReport, error) {
			require.Equal(t, "/env/queries", queriesPath)
			return adapters.ImportReport{}, nil
		},
	}

	cfg := &config.Prest{
		Adapter:     registry,
		QueriesPath: "/ignored",
		PGDatabase:  "prest",
		QueriesConf: config.QueriesConf{
			Storage:         config.QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestNew_AdminQueryStackEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		Adapter:    mock.New(t),
		PGDatabase: "prest",
		QueriesConf: config.QueriesConf{
			RegisterEnabled: true,
			Storage:         config.QueriesStorageDatabase,
		},
	}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
}

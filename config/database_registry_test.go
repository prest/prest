package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestApplyURLToDatabaseConf_RedactsInvalidURLInLog guards against logging a
// raw, credential-bearing connection URL when it fails to parse (an easy
// config-typo trigger, e.g. a non-numeric port). The warning must still
// surface the alias and error for debugging, but the password must never
// appear in the clear in the log line.
//
// Mutates the process-wide slog default logger, so this test cannot run in
// parallel with anything else that does the same.
func TestApplyURLToDatabaseConf_RedactsInvalidURLInLog(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	db := &DatabaseConf{
		Alias: "broken",
		URL:   "postgres://admin:TOP_SECRET_PASSWORD@host:not-a-port/app",
	}
	applyURLToDatabaseConf(db)

	logged := buf.String()
	require.Contains(t, logged, "broken")
	require.NotContains(t, logged, "TOP_SECRET_PASSWORD")
	require.Contains(t, logged, "admin:***@")
}

func resetViperForTest(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestParseDatabaseRegistry_EnvIndexed(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	unsetEnvForTest(t, "DATABASE_ALIAS_2")
	unsetEnvForTest(t, "DATABASE_URL_2")

	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://user:pass@cluster-a.example.com:5432/app_a?sslmode=require")
	t.Setenv("DATABASE_ALIAS_2", "tenant-b")
	t.Setenv("DATABASE_URL_2", "postgres://user:pass@cluster-b.example.com:5432/app_b?sslmode=disable")

	v := viper.New()
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	parseDatabaseRegistry(v, cfg)
	require.Len(t, cfg.Databases, 2)
	require.Equal(t, "tenant-a", cfg.Databases[0].Alias)
	require.Equal(t, "app_a", cfg.Databases[0].Database)
	require.Equal(t, "cluster-a.example.com", cfg.Databases[0].Host)
	require.Equal(t, "require", cfg.Databases[0].SSL.Mode)
}

func TestParseDatabaseRegistry_EnvOverridesTOML(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")

	t.Setenv("PREST_CONF", "../testdata/databases.toml")

	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://override:override@override-host:5432/override_db?sslmode=require")

	v, _ := viperCfg()
	require.NoError(t, v.ReadInConfig())
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	parseDatabaseRegistry(v, cfg)
	require.Len(t, cfg.Databases, 2)
	require.Equal(t, "override-host", cfg.Databases[0].Host)
	require.Equal(t, "override_db", cfg.Databases[0].Database)
	require.Equal(t, "tenant-b", cfg.Databases[1].Alias)
}

func TestParseDatabaseRegistry_LegacyUnchanged(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	unsetEnvForTest(t, "DATABASE_URL")

	t.Setenv("DATABASE_URL", "postgresql://cloud:cloudPass@localhost:5432/CloudDatabase/?sslmode=disable")
	v := viper.New()
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	parseDatabaseRegistry(v, cfg)
	require.Empty(t, cfg.Databases)
	require.Equal(t, "CloudDatabase", cfg.PGDatabase)
}

func TestParseDatabaseRegistry_MissingURL(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	unsetEnvForTest(t, "DATABASE_URL_1")

	v := viper.New()
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	parseDatabaseRegistry(v, cfg)
	require.Empty(t, cfg.Databases)
}

func TestParseDatabaseRegistry_DuplicateAlias(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://user:pass@host:5432/app_a?sslmode=disable")
	t.Setenv("DATABASE_ALIAS_2", "tenant-a")
	t.Setenv("DATABASE_URL_2", "postgres://user:pass@host:5432/app_b?sslmode=disable")

	v := viper.New()
	cfg := &Prest{}
	require.NoError(t, parseDBConfig(v, cfg))
	parseDatabaseRegistry(v, cfg)
	require.Len(t, cfg.Databases, 1)
	require.Equal(t, "tenant-a", cfg.Databases[0].Alias)
	require.Equal(t, "app_a", cfg.Databases[0].Database)
}

func TestHasDatabaseRegistry(t *testing.T) {
	t.Parallel()

	var nilCfg *Prest
	require.False(t, nilCfg.HasDatabaseRegistry())
	require.False(t, (&Prest{}).HasDatabaseRegistry())
	require.True(t, (&Prest{Databases: []DatabaseConf{{Alias: "a"}}}).HasDatabaseRegistry())
}

func Test_fillDatabaseDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		db                *DatabaseConf
		cfg               *Prest
		expectHost        string
		expectPort        int
		expectUser        string
		expectPass        string
		expectDatabase    string
		expectSSLMode     string
		expectSSLCert     string
		expectSSLKey      string
		expectSSLRootCert string
	}{
		{
			name: "all fields empty, filled from defaults",
			db:   &DatabaseConf{},
			cfg: &Prest{
				PGHost:        "default-host",
				PGPort:        5432,
				PGUser:        "default-user",
				PGPass:        "default-pass",
				PGDatabase:    "default-db",
				PGSSLMode:     "require",
				PGSSLCert:     "/path/to/cert.crt",
				PGSSLKey:      "/path/to/key.key",
				PGSSLRootCert: "/path/to/root.crt",
			},
			expectHost:        "default-host",
			expectPort:        5432,
			expectUser:        "default-user",
			expectPass:        "default-pass",
			expectDatabase:    "default-db",
			expectSSLMode:     "require",
			expectSSLCert:     "/path/to/cert.crt",
			expectSSLKey:      "/path/to/key.key",
			expectSSLRootCert: "/path/to/root.crt",
		},
		{
			name: "host already set, not overwritten",
			db: &DatabaseConf{
				Host: "custom-host",
			},
			cfg: &Prest{
				PGHost:     "default-host",
				PGPort:     5432,
				PGUser:     "default-user",
				PGDatabase: "default-db",
			},
			expectHost:     "custom-host",
			expectPort:     5432,
			expectUser:     "default-user",
			expectDatabase: "default-db",
		},
		{
			name: "port already set, not overwritten",
			db: &DatabaseConf{
				Port: 3306,
			},
			cfg: &Prest{
				PGHost:     "default-host",
				PGPort:     5432,
				PGUser:     "default-user",
				PGDatabase: "default-db",
			},
			expectHost:     "default-host",
			expectPort:     3306,
			expectUser:     "default-user",
			expectDatabase: "default-db",
		},
		{
			name: "user already set, not overwritten",
			db: &DatabaseConf{
				User: "custom-user",
			},
			cfg: &Prest{
				PGHost:     "default-host",
				PGPort:     5432,
				PGUser:     "default-user",
				PGDatabase: "default-db",
			},
			expectHost:     "default-host",
			expectPort:     5432,
			expectUser:     "custom-user",
			expectDatabase: "default-db",
		},
		{
			name: "password already set, not overwritten",
			db: &DatabaseConf{
				Pass: "custom-pass",
			},
			cfg: &Prest{
				PGHost:     "default-host",
				PGPort:     5432,
				PGUser:     "default-user",
				PGPass:     "default-pass",
				PGDatabase: "default-db",
			},
			expectHost:     "default-host",
			expectPort:     5432,
			expectUser:     "default-user",
			expectPass:     "custom-pass",
			expectDatabase: "default-db",
		},
		{
			name: "database already set, not overwritten",
			db: &DatabaseConf{
				Database: "custom-db",
			},
			cfg: &Prest{
				PGHost:     "default-host",
				PGPort:     5432,
				PGUser:     "default-user",
				PGDatabase: "default-db",
			},
			expectHost:     "default-host",
			expectPort:     5432,
			expectUser:     "default-user",
			expectDatabase: "custom-db",
		},
		{
			name: "SSL mode already set, not overwritten",
			db: &DatabaseConf{
				SSL: DatabaseSSLConf{Mode: "disable"},
			},
			cfg: &Prest{
				PGHost:    "default-host",
				PGPort:    5432,
				PGUser:    "default-user",
				PGSSLMode: "require",
			},
			expectHost:    "default-host",
			expectPort:    5432,
			expectUser:    "default-user",
			expectSSLMode: "disable",
		},
		{
			name: "SSL cert already set, not overwritten",
			db: &DatabaseConf{
				SSL: DatabaseSSLConf{Cert: "/custom/cert.crt"},
			},
			cfg: &Prest{
				PGHost:    "default-host",
				PGPort:    5432,
				PGUser:    "default-user",
				PGSSLCert: "/default/cert.crt",
			},
			expectHost:    "default-host",
			expectPort:    5432,
			expectUser:    "default-user",
			expectSSLCert: "/custom/cert.crt",
		},
		{
			name: "SSL key already set, not overwritten",
			db: &DatabaseConf{
				SSL: DatabaseSSLConf{Key: "/custom/key.key"},
			},
			cfg: &Prest{
				PGHost:   "default-host",
				PGPort:   5432,
				PGUser:   "default-user",
				PGSSLKey: "/default/key.key",
			},
			expectHost:   "default-host",
			expectPort:   5432,
			expectUser:   "default-user",
			expectSSLKey: "/custom/key.key",
		},
		{
			name: "SSL root cert already set, not overwritten",
			db: &DatabaseConf{
				SSL: DatabaseSSLConf{RootCert: "/custom/root.crt"},
			},
			cfg: &Prest{
				PGHost:        "default-host",
				PGPort:        5432,
				PGUser:        "default-user",
				PGSSLRootCert: "/default/root.crt",
			},
			expectHost:        "default-host",
			expectPort:        5432,
			expectUser:        "default-user",
			expectSSLRootCert: "/custom/root.crt",
		},
		{
			name: "mixed set and unset fields",
			db: &DatabaseConf{
				Host:     "custom-host",
				User:     "custom-user",
				Database: "custom-db",
				SSL: DatabaseSSLConf{
					Mode: "disable",
					Key:  "/custom/key.key",
				},
			},
			cfg: &Prest{
				PGHost:        "default-host",
				PGPort:        5432,
				PGUser:        "default-user",
				PGPass:        "default-pass",
				PGDatabase:    "default-db",
				PGSSLMode:     "require",
				PGSSLCert:     "/default/cert.crt",
				PGSSLKey:      "/default/key.key",
				PGSSLRootCert: "/default/root.crt",
			},
			expectHost:        "custom-host",
			expectPort:        5432,
			expectUser:        "custom-user",
			expectPass:        "default-pass",
			expectDatabase:    "custom-db",
			expectSSLMode:     "disable",
			expectSSLCert:     "/default/cert.crt",
			expectSSLKey:      "/custom/key.key",
			expectSSLRootCert: "/default/root.crt",
		},
		{
			name: "port zero fills from default",
			db: &DatabaseConf{
				Host: "custom-host",
				Port: 0,
			},
			cfg: &Prest{
				PGHost: "default-host",
				PGPort: 5432,
			},
			expectHost: "custom-host",
			expectPort: 5432,
		},
		{
			name: "empty SSL fills from defaults",
			db: &DatabaseConf{
				Host: "custom-host",
				SSL:  DatabaseSSLConf{},
			},
			cfg: &Prest{
				PGHost:        "default-host",
				PGSSLMode:     "require",
				PGSSLCert:     "/default/cert.crt",
				PGSSLKey:      "/default/key.key",
				PGSSLRootCert: "/default/root.crt",
			},
			expectHost:        "custom-host",
			expectSSLMode:     "require",
			expectSSLCert:     "/default/cert.crt",
			expectSSLKey:      "/default/key.key",
			expectSSLRootCert: "/default/root.crt",
		},
		{
			name:              "empty config uses zero values",
			db:                &DatabaseConf{},
			cfg:               &Prest{},
			expectHost:        "",
			expectPort:        0,
			expectUser:        "",
			expectPass:        "",
			expectDatabase:    "",
			expectSSLMode:     "",
			expectSSLCert:     "",
			expectSSLKey:      "",
			expectSSLRootCert: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fillDatabaseDefaults(tt.db, tt.cfg)
			require.Equal(t, tt.expectHost, tt.db.Host, "Host mismatch")
			require.Equal(t, tt.expectPort, tt.db.Port, "Port mismatch")
			require.Equal(t, tt.expectUser, tt.db.User, "User mismatch")
			require.Equal(t, tt.expectPass, tt.db.Pass, "Pass mismatch")
			require.Equal(t, tt.expectDatabase, tt.db.Database, "Database mismatch")
			require.Equal(t, tt.expectSSLMode, tt.db.SSL.Mode, "SSL Mode mismatch")
			require.Equal(t, tt.expectSSLCert, tt.db.SSL.Cert, "SSL Cert mismatch")
			require.Equal(t, tt.expectSSLKey, tt.db.SSL.Key, "SSL Key mismatch")
			require.Equal(t, tt.expectSSLRootCert, tt.db.SSL.RootCert, "SSL RootCert mismatch")
		})
	}
}

func Test_fillDatabaseDefaults_poolLimits(t *testing.T) {
	t.Parallel()

	t.Run("zero fills from global defaults", func(t *testing.T) {
		db := &DatabaseConf{}
		cfg := &Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2}
		fillDatabaseDefaults(db, cfg)
		require.Equal(t, 10, db.MaxOpenConn)
		require.Equal(t, 2, db.MaxIdleConn)
	})

	t.Run("explicit values are preserved", func(t *testing.T) {
		db := &DatabaseConf{MaxOpenConn: 25, MaxIdleConn: 5}
		cfg := &Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2}
		fillDatabaseDefaults(db, cfg)
		require.Equal(t, 25, db.MaxOpenConn)
		require.Equal(t, 5, db.MaxIdleConn)
	})
}

func TestProfileByAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		cfg   *Prest
		alias string
		want  DatabaseConf
		want2 bool
	}{
		{
			name:  "nil config",
			cfg:   nil,
			alias: "any-alias",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name:  "empty databases list",
			cfg:   &Prest{},
			alias: "any-alias",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name:  "empty databases slice",
			cfg:   &Prest{Databases: []DatabaseConf{}},
			alias: "any-alias",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name: "single database, alias matches",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
				},
			},
			alias: "tenant-a",
			want: DatabaseConf{
				Alias:    "tenant-a",
				Host:     "host-a",
				Port:     5432,
				User:     "user-a",
				Database: "db-a",
			},
			want2: true,
		},
		{
			name: "single database, alias does not match",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
				},
			},
			alias: "tenant-b",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name: "multiple databases, first matches",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
					{
						Alias:    "tenant-b",
						Host:     "host-b",
						Port:     5432,
						User:     "user-b",
						Database: "db-b",
					},
					{
						Alias:    "tenant-c",
						Host:     "host-c",
						Port:     5432,
						User:     "user-c",
						Database: "db-c",
					},
				},
			},
			alias: "tenant-a",
			want: DatabaseConf{
				Alias:    "tenant-a",
				Host:     "host-a",
				Port:     5432,
				User:     "user-a",
				Database: "db-a",
			},
			want2: true,
		},
		{
			name: "multiple databases, middle matches",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
					{
						Alias:    "tenant-b",
						Host:     "host-b",
						Port:     5432,
						User:     "user-b",
						Database: "db-b",
					},
					{
						Alias:    "tenant-c",
						Host:     "host-c",
						Port:     5432,
						User:     "user-c",
						Database: "db-c",
					},
				},
			},
			alias: "tenant-b",
			want: DatabaseConf{
				Alias:    "tenant-b",
				Host:     "host-b",
				Port:     5432,
				User:     "user-b",
				Database: "db-b",
			},
			want2: true,
		},
		{
			name: "multiple databases, last matches",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
					{
						Alias:    "tenant-b",
						Host:     "host-b",
						Port:     5432,
						User:     "user-b",
						Database: "db-b",
					},
					{
						Alias:    "tenant-c",
						Host:     "host-c",
						Port:     5432,
						User:     "user-c",
						Database: "db-c",
					},
				},
			},
			alias: "tenant-c",
			want: DatabaseConf{
				Alias:    "tenant-c",
				Host:     "host-c",
				Port:     5432,
				User:     "user-c",
				Database: "db-c",
			},
			want2: true,
		},
		{
			name: "multiple databases, none match",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
					{
						Alias:    "tenant-b",
						Host:     "host-b",
						Port:     5432,
						User:     "user-b",
						Database: "db-b",
					},
				},
			},
			alias: "tenant-x",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name: "empty alias string",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "tenant-a",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
				},
			},
			alias: "",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name: "case sensitivity check",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "Tenant-A",
						Host:     "host-a",
						Port:     5432,
						User:     "user-a",
						Database: "db-a",
					},
				},
			},
			alias: "tenant-a",
			want:  DatabaseConf{},
			want2: false,
		},
		{
			name: "database with SSL configuration",
			cfg: &Prest{
				Databases: []DatabaseConf{
					{
						Alias:    "secure-db",
						Host:     "host-secure",
						Port:     5432,
						User:     "user-secure",
						Database: "db-secure",
						SSL: DatabaseSSLConf{
							Mode:     "require",
							Cert:     "/path/to/cert",
							Key:      "/path/to/key",
							RootCert: "/path/to/root",
						},
					},
				},
			},
			alias: "secure-db",
			want: DatabaseConf{
				Alias:    "secure-db",
				Host:     "host-secure",
				Port:     5432,
				User:     "user-secure",
				Database: "db-secure",
				SSL: DatabaseSSLConf{
					Mode:     "require",
					Cert:     "/path/to/cert",
					Key:      "/path/to/key",
					RootCert: "/path/to/root",
				},
			},
			want2: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got2 := tt.cfg.ProfileByAlias(tt.alias)
			require.Equal(t, tt.want, got, "ProfileByAlias profile mismatch")
			require.Equal(t, tt.want2, got2, "ProfileByAlias found mismatch")
		})
	}
}

func TestFillMySQLDoesNotCopyPostgresDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Prest{
		Engine: EnginePostgres, PGHost: "127.0.0.1", PGPort: 5432,
		PGUser: "postgres", PGPass: "postgres", PGDatabase: "prest", PGSSLMode: "disable",
	}
	db := &DatabaseConf{Alias: "shop", Engine: EngineMySQL, User: "app", Database: "shop"}
	fillDatabaseDefaults(db, cfg)
	require.Empty(t, db.Host)
	require.Equal(t, 3306, db.Port)
	require.Equal(t, "app", db.User)
	require.Empty(t, db.Pass)
	require.Equal(t, "shop", db.Database)
	require.Equal(t, "disable", db.SSL.Mode)
	require.ErrorIs(t, (&Prest{Engine: EngineMySQL, Databases: []DatabaseConf{*db}}).ValidateEngines(), ErrMySQLConfig)
}

func TestApplyMySQLURLDoesNotUsePostgresParser(t *testing.T) {
	t.Parallel()
	db := &DatabaseConf{Alias: "shop", URL: "mysql://app:secret@db.internal:3307/shop?tls=true"}
	applyURLToDatabaseConf(db)
	require.Equal(t, EngineMySQL, db.Engine)
	require.Equal(t, "app", db.User)
	require.Equal(t, "secret", db.Pass)
	require.Equal(t, "db.internal", db.Host)
	require.Equal(t, 3307, db.Port)
	require.Equal(t, "shop", db.Database)
	require.Equal(t, "require", db.SSL.Mode)

	pg := &DatabaseConf{Alias: "pg", Engine: EnginePostgres, URL: "mysql://app:secret@db:3306/shop"}
	applyURLToDatabaseConf(pg)
	require.Empty(t, pg.Host)
	require.Empty(t, pg.User)
}

// Test_parseDatabaseRegistry_envEntryInheritsPoolLimits guards DATABASE_URL_<n>
// entries: without the global limits they reached database/sql as 0, which
// means an unlimited open pool and no idle connections.
func Test_parseDatabaseRegistry_envEntryInheritsPoolLimits(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"postgres", "postgres://user:pass@cluster-a.example.com:5432/app_a?sslmode=disable"},
		{"mysql", "mysql://user:pass@mysql-a.example.com:3306/app_a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetViperForTest(t)
			unsetEnvForTest(t, "DATABASE_ALIAS_2")
			unsetEnvForTest(t, "DATABASE_URL_2")
			t.Setenv("DATABASE_ALIAS_1", "tenant-a")
			t.Setenv("DATABASE_URL_1", tc.url)

			cfg := &Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2}
			parseDatabaseRegistry(viper.New(), cfg)
			require.Len(t, cfg.Databases, 1)
			require.Equal(t, 10, cfg.Databases[0].MaxOpenConn)
			require.Equal(t, 2, cfg.Databases[0].MaxIdleConn)
		})
	}
}

// Test_fillPoolDefaults covers both branches: unset limits take the global
// values and explicit ones are kept.
func Test_fillPoolDefaults(t *testing.T) {
	t.Parallel()

	cfg := &Prest{PGMaxOpenConn: 10, PGMaxIdleConn: 2}

	unset := &DatabaseConf{}
	fillPoolDefaults(unset, cfg)
	require.Equal(t, 10, unset.MaxOpenConn)
	require.Equal(t, 2, unset.MaxIdleConn)

	explicit := &DatabaseConf{MaxOpenConn: 25, MaxIdleConn: 5}
	fillPoolDefaults(explicit, cfg)
	require.Equal(t, 25, explicit.MaxOpenConn)
	require.Equal(t, 5, explicit.MaxIdleConn)

	nilCfg := &DatabaseConf{}
	fillPoolDefaults(nilCfg, nil)
	require.Zero(t, nilCfg.MaxOpenConn)
}

func registryTOML(t *testing.T, body string) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(strings.NewReader(body)))
	return v
}

func unsetRegistryEnv(t *testing.T) {
	t.Helper()
	for i := 1; i <= 3; i++ {
		unsetEnvForTest(t, fmt.Sprintf("DATABASE_ALIAS_%d", i))
		unsetEnvForTest(t, fmt.Sprintf("DATABASE_URL_%d", i))
		unsetEnvForTest(t, fmt.Sprintf("PREST_DATABASE_ALIAS_%d", i))
		unsetEnvForTest(t, fmt.Sprintf("PREST_DATABASE_URL_%d", i))
	}
}

func timezonesByAlias(cfg *Prest) map[string]string {
	out := make(map[string]string, len(cfg.Databases))
	for _, db := range cfg.Databases {
		out[db.Alias] = db.Timezone
	}
	return out
}

// TOML entries: entry URL > entry timezone > global pg.timezone > unset. An
// invalid value at any level falls through to the next one.
func TestParseDatabaseRegistry_TimezoneTOMLPrecedence(t *testing.T) {
	unsetRegistryEnv(t)
	buf := captureSlog(t)
	v := registryTOML(t, `
[[databases]]
alias = "inherits"
database = "a"

[[databases]]
alias = "own"
database = "b"
timezone = "Asia/Tokyo"

[[databases]]
alias = "badown"
database = "c"
timezone = "UTC password=x"

[[databases]]
alias = "url"
url = "postgres://u:p@h:5432/d?TimeZone=Etc/GMT%2B3"
timezone = "Asia/Tokyo"

[[databases]]
alias = "badurl"
url = "postgres://u:p@h:5432/d?timezone=Mars/Olympus"
timezone = "Asia/Tokyo"

[[databases]]
alias = "my"
engine = "mysql"
user = "root"
database = "shop"

[[databases]]
alias = "mybad"
engine = "mysql"
user = "root"
database = "shop"
timezone = "UTC'"

[[databases]]
alias = "myurl"
url = "mysql://root:p@h:3306/shop?timezone=UTC"
`)
	cfg := &Prest{PGTimezone: "America/Sao_Paulo"}
	parseDatabaseRegistry(v, cfg)
	require.Equal(t, map[string]string{
		"inherits": "America/Sao_Paulo",
		"own":      "Asia/Tokyo",
		"badown":   "America/Sao_Paulo",
		"url":      "Etc/GMT+3",
		"badurl":   "Asia/Tokyo",
		"my":       "",
		"mybad":    "",
		"myurl":    "",
	}, timezonesByAlias(cfg))

	logged := buf.String()
	require.Contains(t, logged, `key=databases.timezone alias=badown`)
	require.Contains(t, logged, `key="databases.url timezone" alias=badurl`)
	require.Contains(t, logged, `key=databases.timezone alias=mybad`)
	require.Contains(t, logged, `value="UTC password=x"`, "logged value is quoted")
}

// With no global zone an entry without its own stays unset (server default).
func TestParseDatabaseRegistry_TimezoneUnset(t *testing.T) {
	unsetRegistryEnv(t)
	v := registryTOML(t, "[[databases]]\nalias = \"a\"\ndatabase = \"a\"\n")
	t.Setenv("DATABASE_ALIAS_1", "env")
	t.Setenv("DATABASE_URL_1", "postgres://u:p@h:5432/e")
	cfg := &Prest{}
	parseDatabaseRegistry(v, cfg)
	require.Equal(t, map[string]string{"a": "", "env": ""}, timezonesByAlias(cfg))
}

// Env entries: URL timezone wins; otherwise the global zone is inherited; a
// mysql:// entry is never filled.
func TestParseDatabaseRegistry_TimezoneEnvEntries(t *testing.T) {
	unsetRegistryEnv(t)
	buf := captureSlog(t)
	t.Setenv("DATABASE_ALIAS_1", "inherits")
	t.Setenv("DATABASE_URL_1", "postgres://u:p@h:5432/a")
	t.Setenv("DATABASE_ALIAS_2", "url")
	t.Setenv("DATABASE_URL_2", "postgres://u:p@h:5432/b?timezone=Asia/Tokyo")
	t.Setenv("DATABASE_ALIAS_3", "my")
	t.Setenv("DATABASE_URL_3", "mysql://root:p@h:3306/shop")
	cfg := &Prest{PGTimezone: "UTC"}
	parseDatabaseRegistry(viper.New(), cfg)
	require.Equal(t, map[string]string{
		"inherits": "UTC",
		"url":      "Asia/Tokyo",
		"my":       "",
	}, timezonesByAlias(cfg))
	require.NotContains(t, buf.String(), tzWarn)
}

// An invalid env URL timezone warns and the entry keeps the global zone.
func TestParseDatabaseRegistry_TimezoneEnvInvalidURL(t *testing.T) {
	unsetRegistryEnv(t)
	buf := captureSlog(t)
	t.Setenv("DATABASE_ALIAS_1", "bad")
	t.Setenv("DATABASE_URL_1", "postgres://u:p@h:5432/a?timezone=UTC%27")
	cfg := &Prest{PGTimezone: "UTC"}
	parseDatabaseRegistry(viper.New(), cfg)
	require.Equal(t, map[string]string{"bad": "UTC"}, timezonesByAlias(cfg))
	require.Contains(t, buf.String(), `key="databases.url timezone" alias=bad`)
}

// fillTimezoneDefault: nil cfg is safe, and a mysql entry keeps no zone even
// when one was set by hand.
func Test_fillTimezoneDefault(t *testing.T) {
	db := &DatabaseConf{Alias: "a"}
	fillTimezoneDefault(db, nil)
	require.Empty(t, db.Timezone)

	db = &DatabaseConf{Alias: "a", Timezone: " Asia/Tokyo "}
	fillTimezoneDefault(db, &Prest{PGTimezone: "UTC"})
	require.Equal(t, "Asia/Tokyo", db.Timezone)

	my := &DatabaseConf{Alias: "m", Engine: "mysql"}
	fillTimezoneDefault(my, &Prest{PGTimezone: "UTC"})
	require.Empty(t, my.Timezone)

	// A hand-set zone on a mysql entry is validated (never inherited), so the
	// struct only ever holds validated zones.
	my = &DatabaseConf{Alias: "m", Engine: "mysql", Timezone: "UTC'"}
	fillTimezoneDefault(my, &Prest{PGTimezone: "UTC"})
	require.Empty(t, my.Timezone)
}

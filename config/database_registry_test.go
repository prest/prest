package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

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
	parseDBConfig(v, cfg)
	err := parseDatabaseRegistry(v, cfg)
	require.NoError(t, err)
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
	parseDBConfig(v, cfg)
	err := parseDatabaseRegistry(v, cfg)
	require.NoError(t, err)
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
	parseDBConfig(v, cfg)
	err := parseDatabaseRegistry(v, cfg)
	require.NoError(t, err)
	require.Empty(t, cfg.Databases)
	require.Equal(t, "CloudDatabase", cfg.PGDatabase)
}

func TestParseDatabaseRegistry_MissingURL(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	unsetEnvForTest(t, "DATABASE_URL_1")

	v := viper.New()
	cfg := &Prest{}
	parseDBConfig(v, cfg)
	err := parseDatabaseRegistry(v, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL_1")
}

func TestParseDatabaseRegistry_DuplicateAlias(t *testing.T) {
	resetViperForTest(t)
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://user:pass@host:5432/app_a?sslmode=disable")
	t.Setenv("DATABASE_ALIAS_2", "tenant-a")
	t.Setenv("DATABASE_URL_2", "postgres://user:pass@host:5432/app_b?sslmode=disable")

	v := viper.New()
	cfg := &Prest{}
	parseDBConfig(v, cfg)
	err := parseDatabaseRegistry(v, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestHasDatabaseRegistry(t *testing.T) {
	t.Parallel()

	require.False(t, HasDatabaseRegistry(nil))
	require.False(t, HasDatabaseRegistry(&Prest{}))
	require.True(t, HasDatabaseRegistry(&Prest{Databases: []DatabaseConf{{Alias: "a"}}}))
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
			got, got2 := ProfileByAlias(tt.cfg, tt.alias)
			require.Equal(t, tt.want, got, "ProfileByAlias profile mismatch")
			require.Equal(t, tt.want2, got2, "ProfileByAlias found mismatch")
		})
	}
}

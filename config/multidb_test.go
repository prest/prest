package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMultiDBManager(t *testing.T) {
	t.Run("creates manager with empty databases map", func(t *testing.T) {
		manager := NewMultiDBManager()
		assert.NotNil(t, manager)
		assert.NotNil(t, manager.Databases)
		assert.Empty(t, manager.Databases)
		assert.Empty(t, manager.DefaultDB)
	})
}

func TestDatabaseConfig_GetConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "returns URL when set",
			config: DatabaseConfig{
				URL: "postgres://user:pass@localhost:5432/db?sslmode=disable",
			},
			expected: "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
		{
			name: "builds connection string from fields",
			config: DatabaseConfig{
				User:     "postgres",
				Password: "secret",
				Database: "mydb",
				Host:     "localhost",
				Port:     5432,
				SSLMode:  "disable",
			},
			expected: "user=postgres dbname=mydb host=localhost port=5432 sslmode=disable connect_timeout=0 password=secret",
		},
		{
			name: "builds connection string without password",
			config: DatabaseConfig{
				User:     "postgres",
				Database: "mydb",
				Host:     "localhost",
				Port:     5432,
				SSLMode:  "require",
			},
			expected: "user=postgres dbname=mydb host=localhost port=5432 sslmode=require connect_timeout=0",
		},
		{
			name: "includes SSL certificates when provided",
			config: DatabaseConfig{
				User:        "postgres",
				Database:    "mydb",
				Host:        "localhost",
				Port:        5432,
				SSLMode:     "verify-full",
				SSLCert:     "/path/to/cert.crt",
				SSLKey:      "/path/to/key.pem",
				SSLRootCert: "/path/to/root.crt",
			},
			expected: "user=postgres dbname=mydb host=localhost port=5432 sslmode=verify-full connect_timeout=0 sslcert=/path/to/cert.crt sslkey=/path/to/key.pem sslrootcert=/path/to/root.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetConnectionString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDatabaseConfig_ParseURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedHost  string
		expectedPort  int
		expectedUser  string
		expectedPass  string
		expectedDB    string
		expectedSSL   string
		expectedError bool
	}{
		{
			name:         "parses standard postgres URL",
			url:          "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
			expectedHost: "localhost",
			expectedPort: 5432,
			expectedUser: "user",
			expectedPass: "pass",
			expectedDB:   "mydb",
			expectedSSL:  "disable",
		},
		{
			name:         "parses URL without port (uses default)",
			url:          "postgres://user:pass@localhost/mydb",
			expectedHost: "localhost",
			expectedPort: 0,
			expectedUser: "user",
			expectedPass: "pass",
			expectedDB:   "mydb",
			expectedSSL:  "",
		},
		{
			name:         "parses URL with special characters in password",
			url:          "postgres://user:p%40ss%23word@localhost:5432/mydb",
			expectedHost: "localhost",
			expectedPort: 5432,
			expectedUser: "user",
			expectedPass: "p@ss#word",
			expectedDB:   "mydb",
		},
		{
			name:          "returns error for invalid URL",
			url:           `invalid%+o`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &DatabaseConfig{}
			err := config.ParseURL(tt.url)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedHost, config.Host)
			assert.Equal(t, tt.expectedPort, config.Port)
			assert.Equal(t, tt.expectedUser, config.User)
			assert.Equal(t, tt.expectedPass, config.Password)
			assert.Equal(t, tt.expectedDB, config.Database)
			if tt.expectedSSL != "" {
				assert.Equal(t, tt.expectedSSL, config.SSLMode)
			}
			assert.Equal(t, tt.url, config.URL)
		})
	}
}

func TestMultiDBManager_getDatabaseCountFromEnv(t *testing.T) {
	tests := []struct {
		name            string
		multiNumberEnv  string
		databaseURLEnv  string
		expectedCount   int
		expectedDBCount int
	}{
		{
			name:            "returns count from DATABASE_MULTI_NUMBER",
			multiNumberEnv:  "3",
			expectedCount:   3,
			expectedDBCount: 3,
		},
		{
			name:           "returns 1 when only DATABASE_URL is set",
			databaseURLEnv: "postgres://user:pass@localhost/db",
			expectedCount:  1,
		},
		{
			name:          "returns 0 when no env vars are set",
			expectedCount: 0,
		},
		{
			name:           "returns 0 for invalid DATABASE_MULTI_NUMBER",
			multiNumberEnv: "invalid",
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up env vars
			os.Unsetenv("DATABASE_MULTI_NUMBER")
			os.Unsetenv("DATABASE_URL")

			if tt.multiNumberEnv != "" {
				t.Setenv("DATABASE_MULTI_NUMBER", tt.multiNumberEnv)
			}
			if tt.databaseURLEnv != "" {
				t.Setenv("DATABASE_URL", tt.databaseURLEnv)
			}

			manager := NewMultiDBManager()
			count := manager.getDatabaseCountFromEnv()

			assert.Equal(t, tt.expectedCount, count)
			if tt.expectedDBCount > 0 {
				assert.Equal(t, tt.expectedDBCount, manager.DatabaseCount)
			}
		})
	}
}

func TestMultiDBManager_loadFromEnvURLs(t *testing.T) {
	t.Run("loads multiple databases from environment variables", func(t *testing.T) {
		// Clean up
		for _, env := range []string{"DATABASE_URL", "DATABASE_URL2", "DATABASE_URL3", "DATABASE_URL_NAME", "DATABASE_URL2_NAME", "DATABASE_URL3_NAME"} {
			os.Unsetenv(env)
		}

		t.Setenv("DATABASE_URL", "postgres://user1:pass1@host1:5432/db1?sslmode=disable")
		t.Setenv("DATABASE_URL_NAME", "primary")
		t.Setenv("DATABASE_URL2", "postgres://user2:pass2@host2:5433/db2?sslmode=require")
		t.Setenv("DATABASE_URL2_NAME", "secondary")
		t.Setenv("DATABASE_URL3", "postgres://user3:pass3@host3:5434/db3")
		// DATABASE_URL3_NAME not set, should default to db3

		viper.Reset()
		viper.SetDefault("pg.maxidleconn", 0)
		viper.SetDefault("pg.maxopenconn", 10)
		viper.SetDefault("pg.conntimeout", 10)
		viper.SetDefault("pg.cache", true)

		manager := NewMultiDBManager()
		err := manager.loadFromEnvURLs(3)
		require.NoError(t, err)

		assert.Len(t, manager.Databases, 3)
		assert.Equal(t, "primary", manager.DefaultDB)

		// Check primary database
		db1, exists := manager.GetDatabase("primary")
		require.True(t, exists)
		assert.Equal(t, "host1", db1.Host)
		assert.Equal(t, 5432, db1.Port)
		assert.Equal(t, "user1", db1.User)
		assert.Equal(t, "pass1", db1.Password)
		assert.Equal(t, "db1", db1.Database)

		// Check secondary database
		db2, exists := manager.GetDatabase("secondary")
		require.True(t, exists)
		assert.Equal(t, "host2", db2.Host)
		assert.Equal(t, 5433, db2.Port)
		assert.Equal(t, "user2", db2.User)
		assert.Equal(t, "pass2", db2.Password)
		assert.Equal(t, "db2", db2.Database)

		// Check third database (default name)
		db3, exists := manager.GetDatabase("db3")
		require.True(t, exists)
		assert.Equal(t, "host3", db3.Host)
		assert.Equal(t, 5434, db3.Port)
	})

	t.Run("skips empty URLs", func(t *testing.T) {
		// Clean up
		for _, env := range []string{"DATABASE_URL", "DATABASE_URL2", "DATABASE_URL_NAME"} {
			os.Unsetenv(env)
		}

		t.Setenv("DATABASE_URL", "postgres://user1:pass1@host1:5432/db1")
		t.Setenv("DATABASE_URL_NAME", "primary")
		// DATABASE_URL2 intentionally not set

		viper.Reset()
		viper.SetDefault("pg.maxidleconn", 0)
		viper.SetDefault("pg.maxopenconn", 10)
		viper.SetDefault("pg.conntimeout", 10)
		viper.SetDefault("pg.cache", true)

		manager := NewMultiDBManager()
		err := manager.loadFromEnvURLs(2)
		require.NoError(t, err)

		assert.Len(t, manager.Databases, 1)
		assert.Equal(t, "primary", manager.DefaultDB)
	})
}

func TestMultiDBManager_GetDatabase(t *testing.T) {
	manager := NewMultiDBManager()
	manager.Databases["db1"] = &DatabaseConfig{Name: "db1", Host: "localhost"}
	manager.Databases["db2"] = &DatabaseConfig{Name: "db2", Host: "remote"}

	t.Run("returns existing database", func(t *testing.T) {
		db, exists := manager.GetDatabase("db1")
		assert.True(t, exists)
		assert.NotNil(t, db)
		assert.Equal(t, "localhost", db.Host)
	})

	t.Run("returns false for non-existing database", func(t *testing.T) {
		db, exists := manager.GetDatabase("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, db)
	})
}

func TestMultiDBManager_GetDefaultDatabase(t *testing.T) {
	t.Run("returns default database when set", func(t *testing.T) {
		manager := NewMultiDBManager()
		manager.Databases["primary"] = &DatabaseConfig{Name: "primary", Host: "localhost"}
		manager.DefaultDB = "primary"

		db, exists := manager.GetDefaultDatabase()
		assert.True(t, exists)
		assert.NotNil(t, db)
		assert.Equal(t, "primary", db.Name)
	})

	t.Run("returns false when no default set", func(t *testing.T) {
		manager := NewMultiDBManager()
		manager.Databases["primary"] = &DatabaseConfig{Name: "primary", Host: "localhost"}

		db, exists := manager.GetDefaultDatabase()
		assert.False(t, exists)
		assert.Nil(t, db)
	})
}

func TestMultiDBManager_GetDatabaseNames(t *testing.T) {
	manager := NewMultiDBManager()
	manager.Databases["db1"] = &DatabaseConfig{Name: "db1"}
	manager.Databases["db2"] = &DatabaseConfig{Name: "db2"}
	manager.Databases["db3"] = &DatabaseConfig{Name: "db3"}

	names := manager.GetDatabaseNames()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "db1")
	assert.Contains(t, names, "db2")
	assert.Contains(t, names, "db3")
}

func TestMultiDBManager_HasMultipleDatabases(t *testing.T) {
	t.Run("returns true with multiple databases", func(t *testing.T) {
		manager := NewMultiDBManager()
		manager.Databases["db1"] = &DatabaseConfig{Name: "db1"}
		manager.Databases["db2"] = &DatabaseConfig{Name: "db2"}
		assert.True(t, manager.HasMultipleDatabases())
	})

	t.Run("returns false with single database", func(t *testing.T) {
		manager := NewMultiDBManager()
		manager.Databases["db1"] = &DatabaseConfig{Name: "db1"}
		assert.False(t, manager.HasMultipleDatabases())
	})

	t.Run("returns false with no databases", func(t *testing.T) {
		manager := NewMultiDBManager()
		assert.False(t, manager.HasMultipleDatabases())
	})
}

func TestMultiDBManager_Integration(t *testing.T) {
	t.Run("load from config file with databases section", func(t *testing.T) {
		// Create a temporary config file
		configContent := `[pg]
host = "default-host"
port = 5432
user = "default-user"
pass = "default-pass"
database = "default-db"

[databases.primary]
host = "primary-host"
port = 5433
user = "primary-user"
pass = "primary-pass"
database = "primary-db"
sslmode = "require"

[databases.analytics]
host = "analytics-host"
port = 5434
user = "analytics-user"
database = "analytics-db"
`
		tmpfile, err := os.CreateTemp("", "prest-config-*.toml")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.WriteString(configContent)
		require.NoError(t, err)
		tmpfile.Close()

		// Reset viper and configure with test file
		viper.Reset()
		configFile = tmpfile.Name()
		viper.SetConfigFile(tmpfile.Name())
		viper.SetConfigType("toml")

		// Set defaults
		viper.SetDefault("pg.host", "127.0.0.1")
		viper.SetDefault("pg.port", 5432)
		viper.SetDefault("pg.database", "prest")
		viper.SetDefault("pg.user", "postgres")
		viper.SetDefault("pg.pass", "postgres")
		viper.SetDefault("pg.ssl.mode", "disable")

		err = viper.ReadInConfig()
		require.NoError(t, err)

		manager := NewMultiDBManager()
		err = manager.loadFromConfigFile()
		require.NoError(t, err)

		assert.Len(t, manager.Databases, 2)
		// Default is the first database added to the map
		// (order depends on map iteration, so we just verify it exists)
		assert.Contains(t, []string{"primary", "analytics"}, manager.DefaultDB)

		// Check primary uses its own config
		db1, exists := manager.GetDatabase("primary")
		require.True(t, exists)
		assert.Equal(t, "primary-host", db1.Host)
		assert.Equal(t, 5433, db1.Port)
		assert.Equal(t, "primary-user", db1.User)
		assert.Equal(t, "primary-pass", db1.Password)
		assert.Equal(t, "primary-db", db1.Database)
		assert.Equal(t, "require", db1.SSLMode)

		// Check analytics uses defaults for missing fields
		db2, exists := manager.GetDatabase("analytics")
		require.True(t, exists)
		assert.Equal(t, "analytics-host", db2.Host)
		assert.Equal(t, 5434, db2.Port)
		assert.Equal(t, "analytics-user", db2.User)
		// Uses default password from viper
		assert.Equal(t, "default-pass", db2.Password)
		assert.Equal(t, "analytics-db", db2.Database)
	})
}

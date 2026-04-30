package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style tests for Multi-Tenancy feature

// Feature: Multi-Database Configuration Management
//   As a system administrator
//   I want to configure multiple databases
//   So that I can support multi-tenant applications

func TestMultiTenancyFeature(t *testing.T) {
	t.Run("Scenario: Creating a new MultiDBManager", func(t *testing.T) {
		// Given I need to manage multiple databases
		// When I create a new MultiDBManager
		manager := NewMultiDBManager()

		// Then it should have an initialized databases map
		assert.NotNil(t, manager.Databases, "Manager should have initialized databases map")
		assert.Empty(t, manager.Databases, "Manager should start with empty databases")
		assert.Empty(t, manager.DefaultDB, "Manager should have no default database initially")
	})

	t.Run("Scenario: Adding databases to the manager", func(t *testing.T) {
		// Given a new MultiDBManager
		manager := NewMultiDBManager()

		// When I add a database configuration
		manager.Databases["primary"] = &DatabaseConfig{
			Name:     "primary",
			Host:     "localhost",
			Port:     5432,
			User:     "postgres",
			Database: "prest",
		}
		manager.DefaultDB = "primary"

		// Then I should be able to retrieve it
		db, exists := manager.GetDatabase("primary")
		assert.True(t, exists, "Should find the primary database")
		assert.Equal(t, "localhost", db.Host, "Host should match")
		assert.Equal(t, 5432, db.Port, "Port should match")

		// And the default database should be set
		defaultDB, exists := manager.GetDefaultDatabase()
		assert.True(t, exists, "Should have a default database")
		assert.Equal(t, "primary", defaultDB.Name, "Default database name should match")
	})

	t.Run("Scenario: Managing multiple databases", func(t *testing.T) {
		// Given a MultiDBManager with multiple databases
		manager := NewMultiDBManager()
		manager.Databases["primary"] = &DatabaseConfig{Name: "primary", Host: "host1"}
		manager.Databases["analytics"] = &DatabaseConfig{Name: "analytics", Host: "host2"}
		manager.DefaultDB = "primary"

		// When I check if multiple databases exist
		// Then it should return true
		assert.True(t, manager.HasMultipleDatabases(), "Should have multiple databases")

		// And I should be able to list all database names
		names := manager.GetDatabaseNames()
		assert.Len(t, names, 2, "Should have 2 database names")
		assert.Contains(t, names, "primary", "Should include primary")
		assert.Contains(t, names, "analytics", "Should include analytics")
	})

	t.Run("Scenario: Querying a non-existent database", func(t *testing.T) {
		// Given a MultiDBManager with one database
		manager := NewMultiDBManager()
		manager.Databases["primary"] = &DatabaseConfig{Name: "primary"}

		// When I query for a database that doesn't exist
		db, exists := manager.GetDatabase("nonexistent")

		// Then it should indicate the database doesn't exist
		assert.False(t, exists, "Should not find non-existent database")
		assert.Nil(t, db, "Should return nil for non-existent database")
	})

	t.Run("Scenario: Single database mode", func(t *testing.T) {
		// Given a MultiDBManager with only one database
		manager := NewMultiDBManager()
		manager.Databases["primary"] = &DatabaseConfig{Name: "primary"}

		// When I check if multiple databases exist
		// Then it should return false
		assert.False(t, manager.HasMultipleDatabases(), "Should not have multiple databases with only one")
	})

	t.Run("Scenario: Building connection strings from configuration", func(t *testing.T) {
		tests := []struct {
			name     string
			config   DatabaseConfig
			expected string
		}{
			{
				name: "Full configuration",
				config: DatabaseConfig{
					User:        "admin",
					Password:    "secret123",
					Database:    "myapp",
					Host:        "db.example.com",
					Port:        5432,
					SSLMode:     "require",
					ConnTimeout: 10,
				},
				expected: "user=admin dbname=myapp host=db.example.com port=5432 sslmode=require connect_timeout=10 password=secret123",
			},
			{
				name: "Configuration with SSL certificates",
				config: DatabaseConfig{
					User:        "admin",
					Database:    "myapp",
					Host:        "db.example.com",
					Port:        5432,
					SSLMode:     "verify-full",
					SSLCert:     "/etc/ssl/certs/client.crt",
					SSLKey:      "/etc/ssl/private/client.key",
					SSLRootCert: "/etc/ssl/certs/ca.crt",
					ConnTimeout: 5,
				},
				expected: "user=admin dbname=myapp host=db.example.com port=5432 sslmode=verify-full connect_timeout=5 sslcert=/etc/ssl/certs/client.crt sslkey=/etc/ssl/private/client.key sslrootcert=/etc/ssl/certs/ca.crt",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given a database configuration
				// When I get the connection string
				connStr := tt.config.GetConnectionString()

				// Then it should match the expected format
				assert.Equal(t, tt.expected, connStr)
			})
		}
	})

	t.Run("Scenario: Parsing DATABASE_URL", func(t *testing.T) {
		// Given a DATABASE_URL string
		databaseURL := "postgres://user:password@db.example.com:5432/mydb?sslmode=require"

		// When I parse it into a DatabaseConfig
		config := &DatabaseConfig{}
		err := config.ParseURL(databaseURL)

		// Then it should extract all connection details
		assert.NoError(t, err)
		assert.Equal(t, "user", config.User)
		assert.Equal(t, "password", config.Password)
		assert.Equal(t, "db.example.com", config.Host)
		assert.Equal(t, 5432, config.Port)
		assert.Equal(t, "mydb", config.Database)
		assert.Equal(t, "require", config.SSLMode)
	})
}

// Feature: Environment-based Multi-Database Configuration
//   As a DevOps engineer
//   I want to configure multiple databases via environment variables
//   So that I can use the same application in different environments

func TestEnvironmentBasedConfiguration(t *testing.T) {
	t.Run("Scenario: Loading multiple databases from environment", func(t *testing.T) {
		// This scenario is tested in unit tests, here we document the behavior
		t.Skip("Covered by TestMultiDBManager_loadFromEnvURLs")
	})
}

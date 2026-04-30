package connection

import (
	"context"
	"sync"
	"testing"

	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMultiPool(t *testing.T) {
	t.Run("returns singleton instance", func(t *testing.T) {
		pool1 := GetMultiPool()
		pool2 := GetMultiPool()

		assert.NotNil(t, pool1)
		assert.NotNil(t, pool2)
		assert.Equal(t, pool1, pool2)
		assert.NotNil(t, pool1.Mtx)
		assert.NotNil(t, pool1.DB)
	})
}

func TestGetMulti(t *testing.T) {
	t.Run("uses default database when no ctx dbname", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		manager := config.NewMultiDBManager()
		manager.Databases["db1"] = &config.DatabaseConfig{
			Name: "db1",
		}
		manager.DefaultDB = "db1"

		pool := GetMultiPool()
		pool.Manager = manager

		ctx := context.Background()
		_, err := GetMulti(ctx)
		assert.Error(t, err)
	})

	t.Run("reads dbname from context using pctx.DBNameKey", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		manager := config.NewMultiDBManager()
		manager.Databases["mydb"] = &config.DatabaseConfig{
			Name: "mydb",
		}

		pool := GetMultiPool()
		pool.Manager = manager

		ctx := context.WithValue(context.Background(), pctx.DBNameKey, "mydb")
		_, err := GetMulti(ctx)
		assert.Error(t, err)
	})

	t.Run("returns error for unknown dbname from context", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		manager := config.NewMultiDBManager()
		pool := GetMultiPool()
		pool.Manager = manager

		ctx := context.WithValue(context.Background(), pctx.DBNameKey, "unknown")
		_, err := GetMulti(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestWithDBName(t *testing.T) {
	t.Run("adds database name to context using pctx.DBNameKey", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithDBName(ctx, "mydb")

		dbName, ok := ctx.Value(pctx.DBNameKey).(string)
		assert.True(t, ok)
		assert.Equal(t, "mydb", dbName)
	})

	t.Run("overwrites existing database name in context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithDBName(ctx, "db1")
		ctx = WithDBName(ctx, "db2")

		dbName, ok := ctx.Value(pctx.DBNameKey).(string)
		assert.True(t, ok)
		assert.Equal(t, "db2", dbName)
	})
}

func TestHealthCheck(t *testing.T) {
	t.Run("returns empty map when no connections", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		ctx := context.Background()
		results := HealthCheck(ctx)
		assert.Empty(t, results)
	})
}

func TestGetAllDatabases(t *testing.T) {
	t.Run("returns empty map when no connections", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		dbs := GetAllDatabases()
		assert.Empty(t, dbs)
	})
}

func TestCloseAll(t *testing.T) {
	t.Run("returns nil when no connections", func(t *testing.T) {
		multiPool = nil
		multiOnce = sync.Once{}

		err := CloseAll()
		assert.NoError(t, err)
	})
}

func TestMultiDBManager_Integration(t *testing.T) {
	t.Run("manager can hold multiple database configs", func(t *testing.T) {
		manager := config.NewMultiDBManager()

		manager.Databases["primary"] = &config.DatabaseConfig{
			Name:     "primary",
			Host:     "primary-host",
			Port:     5432,
			User:     "primary-user",
			Password: "primary-pass",
			Database: "primary-db",
			SSLMode:  "disable",
		}

		manager.Databases["analytics"] = &config.DatabaseConfig{
			Name:     "analytics",
			Host:     "analytics-host",
			Port:     5433,
			User:     "analytics-user",
			Password: "analytics-pass",
			Database: "analytics-db",
			SSLMode:  "require",
		}

		manager.DefaultDB = "primary"

		assert.Len(t, manager.Databases, 2)

		db1, exists := manager.GetDatabase("primary")
		require.True(t, exists)
		assert.Equal(t, "primary-host", db1.Host)

		db2, exists := manager.GetDatabase("analytics")
		require.True(t, exists)
		assert.Equal(t, "analytics-host", db2.Host)

		defaultDB, exists := manager.GetDefaultDatabase()
		require.True(t, exists)
		assert.Equal(t, "primary", defaultDB.Name)
	})

	t.Run("manager correctly identifies multiple databases", func(t *testing.T) {
		manager := config.NewMultiDBManager()
		assert.False(t, manager.HasMultipleDatabases())

		manager.Databases["db1"] = &config.DatabaseConfig{Name: "db1"}
		assert.False(t, manager.HasMultipleDatabases())

		manager.Databases["db2"] = &config.DatabaseConfig{Name: "db2"}
		assert.True(t, manager.HasMultipleDatabases())
	})
}

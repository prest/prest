package connection_test

import (
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	helpers.LoadTestConfig(t)
	t.Log("Open connection")
	db, err := postgres.Get()
	if err != nil {
		t.Fatalf("Expected err equal to nil but got %q", err.Error())
	}

	t.Log("Ping Pong")
	err = db.Ping()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}

func TestConnectionPool_Basic(t *testing.T) {
	t.Run("Pool is initially empty", func(t *testing.T) {
		pool := GetPool()
		assert.NotNil(t, pool)
		assert.NotNil(t, pool.Mtx)
		assert.NotNil(t, pool.DB)
	})
}

func TestGetURI(t *testing.T) {
	originalConf := config.PrestConf
	defer func() { config.PrestConf = originalConf }()

	config.PrestConf = &config.Prest{
		PGUser:        "testuser",
		PGDatabase:    "testdb",
		PGHost:        "localhost",
		PGPort:        5432,
		PGSSLMode:     "disable",
		PGConnTimeout: 10,
	}

	t.Run("Builds URI from default config when DBName is empty", func(t *testing.T) {
		uri := GetURI("")
		assert.NotEmpty(t, uri)
		assert.Contains(t, uri, "user=")
		assert.Contains(t, uri, "dbname=")
		assert.Contains(t, uri, "host=")
		assert.Contains(t, uri, "port=")
	})

	t.Run("Builds URI for specific database", func(t *testing.T) {
		uri := GetURI("mydb")
		assert.NotEmpty(t, uri)
		assert.Contains(t, uri, "dbname=mydb")
	})
}

func TestMustGet(t *testing.T) {
	helpers.LoadTestConfig(t)
	t.Log("Open connection")
	db := postgres.MustGet()
	if db == nil {
		t.Fatalf("expected db connection, but no was!")
	}

	t.Log("Ping Pong")
	err := db.Ping()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
func TestSetAndGetDatabase(t *testing.T) {
	t.Run("Set and get current database", func(t *testing.T) {
		SetDatabase("testdb")
		db := GetDatabase()
		assert.Equal(t, "testdb", db)
	})

	t.Run("Overwrite existing database", func(t *testing.T) {
		SetDatabase("db1")
		SetDatabase("db2")
		db := GetDatabase()
		assert.Equal(t, "db2", db)
	})
}

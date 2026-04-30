package connection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
)

type MultiDBPool struct {
	Mtx     *sync.RWMutex
	DB      map[string]*sqlx.DB
	Manager *config.MultiDBManager
}

var (
	multiPool *MultiDBPool
	multiOnce sync.Once
)

func GetMultiPool() *MultiDBPool {
	multiOnce.Do(func() {
		multiPool = &MultiDBPool{
			Mtx: &sync.RWMutex{},
			DB:  make(map[string]*sqlx.DB),
		}
	})
	return multiPool
}

func InitMultiPool(manager *config.MultiDBManager) error {
	pool := GetMultiPool()
	pool.Mtx.Lock()
	defer pool.Mtx.Unlock()

	pool.Manager = manager

	for name, dbConfig := range manager.Databases {
		if err := pool.createConnection(name, dbConfig); err != nil {
			return fmt.Errorf("failed to create connection for database %s: %w", name, err)
		}
	}

	return nil
}

func (p *MultiDBPool) createConnection(name string, dbConfig *config.DatabaseConfig) error {
	db, err := sqlx.Connect("postgres", dbConfig.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to database %s: %w", name, err)
	}

	db.SetMaxIdleConns(dbConfig.MaxIdleConn)
	db.SetMaxOpenConns(dbConfig.MaxOpenConn)

	if err := db.Ping(); err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return fmt.Errorf("failed to ping database %s: %w (close error: %v)", name, err, closeErr)
		}
		return fmt.Errorf("failed to ping database %s: %w", name, err)
	}

	p.DB[name] = db
	slog.Info("database connection established", "database", name, "host", dbConfig.Host)
	return nil
}

func GetMulti(ctx context.Context) (*sqlx.DB, error) {
	dbName, ok := ctx.Value(pctx.DBNameKey).(string)
	if !ok || dbName == "" {
		pool := GetMultiPool()
		if pool.Manager != nil {
			if defaultDB, exists := pool.Manager.GetDefaultDatabase(); exists {
				dbName = defaultDB.Name
			}
		}
		if dbName == "" {
			return Get()
		}
	}
	return GetFromMultiPool(dbName)
}

func GetFromMultiPool(dbName string) (*sqlx.DB, error) {
	pool := GetMultiPool()

	pool.Mtx.RLock()
	db, exists := pool.DB[dbName]
	pool.Mtx.RUnlock()

	if exists {
		return db, nil
	}

	pool.Mtx.Lock()
	defer pool.Mtx.Unlock()

	if db, exists := pool.DB[dbName]; exists {
		return db, nil
	}

	if pool.Manager == nil {
		return nil, fmt.Errorf("database %s not found and no manager configured", dbName)
	}

	dbConfig, exists := pool.Manager.GetDatabase(dbName)
	if !exists {
		return nil, fmt.Errorf("database %s not found in configuration", dbName)
	}

	if err := pool.createConnection(dbName, dbConfig); err != nil {
		return nil, err
	}

	return pool.DB[dbName], nil
}

func GetAllDatabases() map[string]*sqlx.DB {
	pool := GetMultiPool()
	pool.Mtx.RLock()
	defer pool.Mtx.RUnlock()

	result := make(map[string]*sqlx.DB)
	for name, db := range pool.DB {
		result[name] = db
	}
	return result
}

func CloseAll() error {
	pool := GetMultiPool()
	pool.Mtx.Lock()
	defer pool.Mtx.Unlock()

	var errs []error
	for name, db := range pool.DB {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database %s: %w", name, err))
		} else {
			slog.Info("database connection closed", "database", name)
		}
	}

	pool.DB = make(map[string]*sqlx.DB)

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func HealthCheck(ctx context.Context) map[string]error {
	pool := GetMultiPool()
	pool.Mtx.RLock()
	defer pool.Mtx.RUnlock()

	results := make(map[string]error)
	for name, db := range pool.DB {
		if err := db.PingContext(ctx); err != nil {
			results[name] = err
		}
	}
	return results
}

func GetDBFromContext(ctx context.Context) (*sqlx.DB, error) {
	dbName, ok := ctx.Value(pctx.DBNameKey).(string)
	if !ok || dbName == "" {
		return GetMulti(ctx)
	}
	return GetFromMultiPool(dbName)
}

func WithDBName(ctx context.Context, dbName string) context.Context {
	return context.WithValue(ctx, pctx.DBNameKey, dbName)
}

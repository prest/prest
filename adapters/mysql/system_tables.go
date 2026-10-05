package mysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

func (a *Adapter) EnsureAuthTable(ctx context.Context) error {
	schema, table := "public", "prest_users"
	if a.cfg != nil {
		if a.cfg.AuthSchema != "" {
			schema = a.cfg.AuthSchema
		}
		if a.cfg.AuthTable != "" {
			table = a.cfg.AuthTable
		}
	}
	schemaQ, err := quoteIdent(schema)
	if err != nil {
		return err
	}
	tableQ, err := quoteIdent(table)
	if err != nil {
		return err
	}
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
  `+"`id` BIGINT AUTO_INCREMENT PRIMARY KEY"+`,
  `+"`name` TEXT"+`,
  `+"`username` TEXT"+`,
  `+"`password` TEXT"+`,
  `+"`metadata` JSON"+`,
  UNIQUE KEY `+"`username` (`username`(255))"+`
)`, schemaQ, tableQ))
	return err
}

func (a *Adapter) EnsureQueriesTable(ctx context.Context) error {
	schema, table := a.queriesTable()
	schemaQ, err := quoteIdent(schema)
	if err != nil {
		return err
	}
	tableQ, err := quoteIdent(table)
	if err != nil {
		return err
	}
	index, err := quoteIdent(table + "_location_idx")
	if err != nil {
		return err
	}
	db, err := a.dbFromCtx(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
  `+"`id` BIGINT AUTO_INCREMENT PRIMARY KEY"+`,
  `+"`database_alias` VARCHAR(255) NOT NULL DEFAULT ''"+`,
  `+"`location` VARCHAR(255) NOT NULL"+`,
  `+"`name` VARCHAR(255) NOT NULL"+`,
  `+"`read_sql` TEXT"+`,
  `+"`write_sql` TEXT"+`,
  `+"`update_sql` TEXT"+`,
  `+"`delete_sql` TEXT"+`,
  `+"`description` TEXT"+`,
  `+"`created_by` TEXT"+`,
  `+"`created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)"+`,
  `+"`updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)"+`,
  UNIQUE KEY `+"`identity` (`database_alias`, `location`, `name`)"+`
)`, schemaQ, tableQ))
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE INDEX %s ON %s.%s (`location`)", index, schemaQ, tableQ))
	if isDuplicateIndex(err) {
		return nil
	}
	return err
}

func isDuplicateIndex(err error) bool {
	var me *mysql.MySQLError
	return err != nil && errors.As(err, &me) && me.Number == 1061
}

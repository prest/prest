package cmd

import (
	"fmt"
	"os"

	"github.com/prest/prest/v2/app"

	"github.com/lib/pq"
	"github.com/spf13/cobra"
)

var queriesUpCmd = &cobra.Command{
	Use:   "queries",
	Short: "Create queries table",
	Long:  "Create table used for database-backed custom SQL scripts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := configFrom(cmd)
		db, err := app.PostgresDB(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return fmt.Errorf("acquire database connection for queries create: %w", err)
		}
		if err := app.EnsureQueriesTable(cfg, db); err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return fmt.Errorf("create queries table %s.%s: %w", cfg.QueriesConf.Schema, cfg.QueriesConf.Table, err)
		}
		return nil
	},
}

var queriesDownCmd = &cobra.Command{
	Use:   "queries",
	Short: "Drop queries table",
	Long:  "Drop table used for database-backed custom SQL scripts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := configFrom(cmd)

		db, err := app.PostgresDB(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return fmt.Errorf("acquire database connection for queries drop: %w", err)
		}
		_, err = db.Exec(fmt.Sprintf(
			"DROP TABLE IF EXISTS %s.%s",
			pq.QuoteIdentifier(cfg.QueriesConf.Schema),
			pq.QuoteIdentifier(cfg.QueriesConf.Table),
		))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return fmt.Errorf("drop queries table %s.%s: %w", cfg.QueriesConf.Schema, cfg.QueriesConf.Table, err)
		}
		return nil
	},
}

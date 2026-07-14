package cmd

import (
	"fmt"
	"os"

	"github.com/prest/prest/v2/app"

	"github.com/lib/pq"
	"github.com/spf13/cobra"
)

var authUpCmd = &cobra.Command{
	Use:   "auth",
	Short: "Create auth table",
	Long:  "Create basic table to use on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := configFrom(cmd)
		db, err := app.PostgresDB(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		if err := app.EnsureAuthTable(cfg, db); err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		return nil
	},
}

var authDownCmd = &cobra.Command{
	Use:   "auth",
	Short: "Drop auth table",
	Long:  "Drop basic table used on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := configFrom(cmd)

		db, err := app.PostgresDB(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", pq.QuoteIdentifier(cfg.AuthSchema), pq.QuoteIdentifier(cfg.AuthTable)))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		return nil
	},
}

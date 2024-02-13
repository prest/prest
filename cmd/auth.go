package cmd

import (
	"fmt"
	"os"

	"github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/prest/prest/adapters"
)

var authUpCmd = &cobra.Command{
	Use:   "auth",
	Short: "Create auth table",
	Long:  "Create basic table to use on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		adpt, err := adapters.New(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		db, err := adpt.GetConn()
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		_, err = db.ExecContext(cmd.Context(),
			fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s.%s (id serial, name text, username text unique, password text, metadata jsonb)",
				pq.QuoteIdentifier(cfg.AuthSchema),
				pq.QuoteIdentifier(cfg.AuthTable)))
		if err != nil {
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
		adpt, err := adapters.New(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		db, err := adpt.GetConn()
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		_, err = db.ExecContext(cmd.Context(), fmt.Sprintf(
			"DROP TABLE IF EXISTS %s.%s",
			pq.QuoteIdentifier(cfg.AuthSchema),
			pq.QuoteIdentifier(cfg.AuthTable)))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		return nil
	},
}

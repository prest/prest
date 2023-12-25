package cmd

import (
	"fmt"
	"os"

	"github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/config"
)

var authUpCmd = &cobra.Command{
	Use:   "auth",
	Short: "Create auth table",
	Long:  "Create basic table to use on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.New()
		adpt, err := adapters.New(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		tx, err := adpt.GetTransactionCtx(cmd.Context())
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		_, err = tx.Exec(fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s.%s (id serial, name text, username text unique, password text, metadata jsonb)",
			pq.QuoteIdentifier(config.PrestConf.AuthSchema),
			pq.QuoteIdentifier(config.PrestConf.AuthTable)))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		err = tx.Commit()
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
		cfg := config.New()
		adpt, err := adapters.New(cfg)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		tx, err := adpt.GetTransactionCtx(cmd.Context())
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		_, err = tx.Exec(fmt.Sprintf(
			"DROP TABLE IF EXISTS %s.%s",
			pq.QuoteIdentifier(config.PrestConf.AuthSchema),
			pq.QuoteIdentifier(config.PrestConf.AuthTable)))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		err = tx.Commit()
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}

		return nil
	},
}

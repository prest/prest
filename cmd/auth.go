package cmd

import (
	"fmt"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/spf13/cobra"
	"os"
)

var authUpCmd = &cobra.Command{
	Use: "auth",
	Short: "Create auth table",
	Long: "Create basic table to use on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.PrestConf.Adapter == nil {
			postgres.Load()
		}

		db, err := postgres.Get()
		if err != nil {
			fmt.Fprintf(os.Stdout, err.Error())
			return err
		}
		_, err = db.Exec("CREATE TABLE IF NOT EXISTS public.users (id serial, name text, email text unique, password text, metadata jsonb)")
		if err != nil {
			fmt.Fprintf(os.Stdout, err.Error())
			return err
		}

		return nil
	},
}

var authDownCmd = &cobra.Command{
	Use: "auth",
	Short: "Drop auth table",
	Long: "Drop basic table used on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.PrestConf.Adapter == nil {
			postgres.Load()
		}

		db, err := postgres.Get()
		if err != nil {
			fmt.Fprintf(os.Stdout, err.Error())
			return err
		}
		_, err = db.Exec("DROP TABLE IF EXISTS public.users")
		if err != nil {
			fmt.Fprintf(os.Stdout, err.Error())
			return err
		}

		return nil
	},
}

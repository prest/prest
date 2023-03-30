package cmd

import (
	"fmt"
	"os"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/spf13/cobra"
)

var authUpCmd = &cobra.Command{
	Use:   "auth",
	Short: "Create auth table",
	Long:  "Create basic table to use on auth endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.PrestConf.Adapter == nil {
			postgres.Load()
		}
		db, err := postgres.Get()
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		_, err = db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id serial, name text, username text unique, password text, metadata jsonb)", config.PrestConf.AuthSchema, config.PrestConf.AuthTable))
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
		if config.PrestConf.Adapter == nil {
			postgres.Load()
		}

		db, err := postgres.Get()
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", config.PrestConf.AuthSchema, config.PrestConf.AuthTable))
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return err
		}
		return nil
	},
}

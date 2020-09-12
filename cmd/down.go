package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gosidekick/migration/v2"
	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back all migrations",
	Long:  `Roll back all migrations`,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(context.Background(), path, urlConn, "down")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "exec migrations located in %v\n", path)
		fmt.Fprintf(os.Stdout, "executed %v migrations\n", n)
		for _, e := range executed {
			fmt.Fprintf(os.Stdout, "%v SUCCESS\n", e)
		}
		return nil
	},
}

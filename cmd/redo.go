package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
)

// redoCmd represents the redo command
var redoCmd = &cobra.Command{
	Use:     "redo",
	Short:   "roll back the most recently applied migration, then run it again.",
	Long:    `roll back the most recently applied migration, then run it again.`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(context.Background(), path, urlConn, "down 1")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "exec migrations located in %v\n", path)
		fmt.Fprintf(os.Stdout, "executed %v migrations\n", n)
		for _, e := range executed {
			fmt.Fprintf(os.Stdout, "%v SUCCESS\n", e)
		}
		n, executed, err = migration.Run(context.Background(), path, urlConn, "up")
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

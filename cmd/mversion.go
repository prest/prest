package cmd

import (
	"fmt"
	"os"

	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
)

// mversionCmd represents the version command
var mversionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Show the current migration version",
	Long:    `Show the current migration version`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(cmd.Context(), path, urlConn, "status")
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "check migrations located in %v\n", path)
		fmt.Fprintf(os.Stdout, "%v needs to be executed\n", n)
		for _, e := range executed {
			fmt.Fprintf(os.Stdout, "%v\n", e)
		}
		return nil
	},
}

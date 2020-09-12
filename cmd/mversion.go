package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gosidekick/migration/v2"
	"github.com/spf13/cobra"
)

// mversionCmd represents the version command
var mversionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current migration version",
	Long:  `Show the current migration version`,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(context.Background(), path, urlConn, "status")
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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gosidekick/migration/v2"
	"github.com/spf13/cobra"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all available migrations",
	Long:  `Apply all available migrations`,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(context.Background(), path, urlConn, "up")
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

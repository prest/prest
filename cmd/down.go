package cmd

import (
	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:     "down",
	Short:   "Roll back all migrations",
	Long:    `Roll back all migrations`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(cmd.Context(), path, urlConn, "down")
		if err != nil {
			return err
		}
		slog.Printf("exec migrations located in %v\n", path)
		slog.Printf("executed %v migrations\n", n)
		for _, e := range executed {
			slog.Printf("%v SUCCESS\n", e)
		}
		return nil
	},
}

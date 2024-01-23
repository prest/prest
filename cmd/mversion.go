package cmd

import (
	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
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
			slog.Errorln("exec migrations failed: ", err)
			return err
		}
		slog.Printf("check migrations located in %v\n", path)
		slog.Printf("%v needs to be executed\n", n)
		for _, e := range executed {
			slog.Printf("%v executed\n", e)
		}
		return nil
	},
}

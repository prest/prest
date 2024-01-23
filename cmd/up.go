package cmd

import (
	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:     "up",
	Short:   "Apply all available migrations",
	Long:    `Apply all available migrations`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		n, executed, err := migration.Run(cmd.Context(), path, urlConn, "up")
		if err != nil {
			slog.Errorln("exec migrations failed: ", err)
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

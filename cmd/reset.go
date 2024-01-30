package cmd

import (
	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:     "reset",
	Short:   "Run down and then up command",
	Long:    `Run down and then up command`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		n, executed, err := migration.Run(ctx, path, urlConn, "down")
		if err != nil {
			slog.Errorln("exec migrations failed: ", err)
			return err
		}

		slog.Printf("exec migrations located in %v\n", path)
		slog.Printf("executed %v migrations\n", n)
		for _, e := range executed {
			slog.Printf("%v SUCCESS\n", e)
		}

		n, executed, err = migration.Run(ctx, path, urlConn, "up")
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

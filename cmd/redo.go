package cmd

import (
	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// redoCmd represents the redo command
var redoCmd = &cobra.Command{
	Use:     "redo",
	Short:   "roll back the most recently applied migration, then run it again.",
	Long:    `roll back the most recently applied migration, then run it again.`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		n, executed, err := migration.Run(ctx, path, urlConn, "down 1")
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

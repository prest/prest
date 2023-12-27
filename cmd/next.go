package cmd

import (
	"fmt"
	"strings"

	"github.com/gosidekick/migration/v3"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// nextCmd represents the next command
var nextCmd = &cobra.Command{
	Use:     "next",
	Short:   "Apply the next n migrations",
	Long:    `Apply the next n migrations`,
	PreRunE: checkTable,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("invalid arguments %v", args)
		}
		var (
			n        int
			executed []string
			err      error
		)
		a := args[0]
		if strings.HasPrefix(a, "+") {
			n, executed, err = migration.Run(cmd.Context(), path, urlConn, "up "+strings.TrimPrefix(a, "+"))
		} else {
			n, executed, err = migration.Run(cmd.Context(), path, urlConn, "down "+strings.TrimPrefix(a, "-"))
		}
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

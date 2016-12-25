package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
)

// redoCmd represents the redo command
var redoCmd = &cobra.Command{
	Use:   "redo",
	Short: "roll back the most recently applied migration, then run it again.",
	Long:  `roll back the most recently applied migration, then run it again.`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Redo(pipe, url, path)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}
	},
}

func init() {
	migrateCmd.AddCommand(redoCmd)
}

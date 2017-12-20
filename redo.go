package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/migrate"
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
		go migrate.Redo(pipe, urlConn, path)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}
	},
}

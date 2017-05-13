package cmd

import (
	"os"
	"time"

	"github.com/mattes/migrate/migrate"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Run down and then up command",
	Long:  `Run down and then up command`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Reset(pipe, url, path)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}
	},
}

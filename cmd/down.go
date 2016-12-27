package cmd

import (
	"os"
	"time"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back all migrations",
	Long:  `Roll back all migrations`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Down(pipe, url, path)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}
	},
}

func init() {
	// prest migrate down
	migrateCmd.AddCommand(downCmd)
}

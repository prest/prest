package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/migrate"
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
		go migrate.Down(pipe, urlConn, path)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}
	},
}

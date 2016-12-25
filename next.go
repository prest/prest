package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mattes/migrate/migrate"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/spf13/cobra"
)

// nextCmd represents the next command
var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Apply the next n migrations",
	Long:  `Apply the next n migrations`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		relativeN := args[0]
		relativeNInt, err := strconv.Atoi(relativeN)
		if err != nil {
			fmt.Println("Unable to parse param <n>.")
			os.Exit(1)
		}
		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Migrate(pipe, url, path, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
	},
}

func init() {
	migrateCmd.AddCommand(nextCmd)
}

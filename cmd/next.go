package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/migrate"
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
		go migrate.Migrate(pipe, urlConn, path, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(1)
		}
	},
}

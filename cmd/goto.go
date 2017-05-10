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

// gotoCmd represents the goto command
var gotoCmd = &cobra.Command{
	Use:   "goto",
	Short: "Go to specific migration",
	Long:  `Go to specific migration`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		toVersion := args[0]
		toVersionInt, err := strconv.Atoi(toVersion)
		if err != nil || toVersionInt < 0 {
			fmt.Println("Unable to parse param <v>.")
			os.Exit(1)
		}

		currentVersion, err := migrate.Version(url, path)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		relativeNInt := toVersionInt - int(currentVersion)

		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Migrate(pipe, url, path, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}

	},
}

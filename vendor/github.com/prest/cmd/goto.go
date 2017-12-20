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

		currentVersion, err := migrate.Version(urlConn, path)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}

		relativeNInt := toVersionInt - int(currentVersion)

		timerStart = time.Now()
		pipe := migrate.NewPipe()
		go migrate.Migrate(pipe, urlConn, path, relativeNInt)
		ok := writePipe(pipe)
		printTimer()
		if !ok {
			os.Exit(-1)
		}

	},
}

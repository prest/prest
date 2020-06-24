package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/migrate"
)

// mversionCmd represents the version command
var mversionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current migration version",
	Long:  `Show the current migration version`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		version, err := migrate.Version(urlConn, path)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		fmt.Println(version)
	},
}

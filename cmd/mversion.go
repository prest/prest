package cmd

import (
	"fmt"
	"os"

	"github.com/mattes/migrate/migrate"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/spf13/cobra"
)

// mversionCmd represents the version command
var mversionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current migration version",
	Long:  `Show the current migration version`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		version, err := migrate.Version(url, path)
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		fmt.Println(version)
	},
}

func init() {
	migrateCmd.AddCommand(mversionCmd)
}

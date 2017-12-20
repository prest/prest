package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/migrate"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create new migration file in path",
	Long:  `Create new migration file in path`,
	Run: func(cmd *cobra.Command, args []string) {
		verifyMigrationsPath(path)
		if len(args) < 1 {
			fmt.Println("Please specify name.")
			os.Exit(-1)
		}
		name := args[0]
		migrationFile, err := migrate.Create(urlConn, path, name)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Version %v migration files created in %v:\n", migrationFile.Version, path)
		fmt.Println(migrationFile.UpFile.FileName)
		fmt.Println(migrationFile.DownFile.FileName)
	},
}

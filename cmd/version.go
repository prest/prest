package cmd

import (
	"fmt"

	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/nuveo/prest/helpers"
	"github.com/spf13/cobra"
)

// versionCmd show version pREST
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of pREST",
	Long:  `All software has versions. This is pREST's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Serve a RESTful API from any PostgreSQL database", helpers.PrestReleaseVersion())
	},
}

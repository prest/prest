package cmd

import (
	"fmt"

	"github.com/prest/helpers"
	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
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

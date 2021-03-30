package cmd

import (
	"fmt"

	"github.com/prest/prest/helpers"
	"github.com/spf13/cobra"
)

// versionCmd show version pREST
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of pREST",
	Long:  `All software has versions. This is pREST's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new", helpers.PrestReleaseVersion())
	},
}

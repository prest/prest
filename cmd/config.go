package cmd

import (
	"fmt"

	"github.com/prest/prest/v2/config"

	"github.com/spf13/cobra"
)

var (
	exportJSONFile         string
	exportJSONPretty       bool
	exportJSONIncludeInfra bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Work with the pREST configuration",
	Long:  "Inspect and convert the pREST configuration",
}

var configExportJSONCmd = &cobra.Command{
	Use:   "export-json",
	Short: "Print prest.toml as a PREST_CONFIG_JSON document",
	Long: `Print the application config set in a prest.toml file as one JSON document,
ready for the PREST_CONFIG_JSON env var.

The http, https, engine, pg, mysql and databases sections and queries.location
are left out: set them with their own env vars. Defaults and PREST_* env vars
are not exported, only what the file sets.

The output can hold secrets such as jwt.key. Store it as a secret.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := config.ExportConfigJSON(exportJSONFile, exportJSONPretty, exportJSONIncludeInfra)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return err
	},
}

func init() {
	flags := configExportJSONCmd.Flags()
	flags.StringVarP(&exportJSONFile, "file", "f", "", "TOML config file to export (default $PREST_CONF, else ./prest.toml)")
	flags.BoolVar(&exportJSONPretty, "pretty", false, "Indent the JSON output")
	flags.BoolVar(&exportJSONIncludeInfra, "include-infra", false, "Also export the infra sections, for debugging only")
}

package cmd

import (
	"fmt"
	"net/url"

	"github.com/prest/prest/config"
	"github.com/spf13/cobra"

	// pq driver
	_ "github.com/lib/pq"
)

var (
	urlConn string
	path    string
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Execute migration operations",
	Long:  `Execute migration operations`,
}

func driverURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&sslcert=%s&sslkey=%s&sslrootcert=%s",
		url.PathEscape(config.PrestConf.PGUser),
		url.PathEscape(config.PrestConf.PGPass),
		url.PathEscape(config.PrestConf.PGHost),
		config.PrestConf.PGPort,
		url.PathEscape(config.PrestConf.PGDatabase),
		url.QueryEscape(config.PrestConf.SSLMode),
		url.QueryEscape(config.PrestConf.SSLCert),
		url.QueryEscape(config.PrestConf.SSLKey),
		url.QueryEscape(config.PrestConf.SSLRootCert))

}

package cmd

import (
	"fmt"
	"net/url"

	"github.com/prest/prest/adapters/postgres"
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

func checkTable(cmd *cobra.Command, args []string) error {
	if config.PrestConf.Adapter == nil {
		postgres.Load()
	}
	sc := config.PrestConf.Adapter.ShowTable("public", "schema_migrations")
	if err := sc.Err(); err != nil {
		return err
	}
	ts := []struct {
		ColName string `json:"column_name,omitempty"`
	}{}
	_, err := sc.Scan(&ts)
	if err != nil {
		return err
	}
	var index *int
	for i := range ts {
		if ts[i].ColName == "dirty" {
			index = &i
			break
		}
	}
	if index != nil {
		db, err := postgres.Get()
		if err != nil {
			return err
		}
		_, err = db.Exec("ALTER TABLE public.schema_migrations DROP COLUMN dirty")
		if err != nil {
			return err
		}
	}
	return nil
}

func driverURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&sslcert=%s&sslkey=%s&sslrootcert=%s",
		url.PathEscape(config.PrestConf.PGUser),
		url.PathEscape(config.PrestConf.PGPass),
		url.PathEscape(config.PrestConf.PGHost),
		config.PrestConf.PGPort,
		url.PathEscape(config.PrestConf.PGDatabase),
		url.QueryEscape(config.PrestConf.PGSSLMode),
		url.QueryEscape(config.PrestConf.PGSSLCert),
		url.QueryEscape(config.PrestConf.PGSSLKey),
		url.QueryEscape(config.PrestConf.PGSSLRootCert))
}

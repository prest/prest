package cmd

import (
	"fmt"
	"net/url"

	// load pq driver
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"

	"github.com/prest/prest/adapters"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Execute migration operations",
	Long:  `Execute migration operations`,
}

func checkTable(cmd *cobra.Command, args []string) error {
	if path == "" {
		return ErrPathNotSet
	}
	if urlConn == "" {
		return ErrURLNotSet
	}
	cmd.SilenceUsage = true

	adpt, err := adapters.New(cfg)
	if err != nil {
		slog.Errorln("checkTable adapters error: ", err)
		return err
	}

	sc := adpt.ShowTable("public", "schema_migrations")
	if err := sc.Err(); err != nil {
		slog.Errorln("checkTable ShowTable error: ", err)
		return err
	}
	ts := []struct {
		ColName string `json:"column_name,omitempty"`
	}{}
	_, err = sc.Scan(&ts)
	if err != nil {
		slog.Errorln("checkTable Scan error: ", err)
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
		db, err := adpt.GetConn()
		if err != nil {
			slog.Errorln("checkTable GetConn error: ", err)
			return err
		}
		_, err = db.ExecContext(cmd.Context(),
			"ALTER TABLE public.schema_migrations DROP COLUMN dirty")
		if err != nil {
			slog.Errorln("checkTable Exec error: ", err)
			return err
		}
	}
	return nil
}

func driverURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&sslcert=%s&sslkey=%s&sslrootcert=%s",
		url.PathEscape(cfg.PGUser),
		url.PathEscape(cfg.PGPass),
		url.PathEscape(cfg.PGHost),
		cfg.PGPort,
		url.PathEscape(cfg.PGDatabase),
		url.QueryEscape(cfg.PGSSLMode),
		url.QueryEscape(cfg.PGSSLCert),
		url.QueryEscape(cfg.PGSSLKey),
		url.QueryEscape(cfg.PGSSLRootCert))
}

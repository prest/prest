package cmd

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"

	"github.com/spf13/cobra"
	// pq driver
	_ "github.com/lib/pq"
)

var (
	urlConn string
	path    string
)

var (
	ErrPathNotSet = errors.New("Migrations path not set. \nPlease set it using --path flag or in your prest config file")
	ErrURLNotSet  = errors.New("Database URL not set. \nPlease set it using --url flag or configure it on your prest config file")
)

func init() {
	// Empty default, not a DSN built from cfg: a credential-bearing DSN must
	// never be baked into a cobra flag default, since cobra prints defaults
	// verbatim in --help. resolveURLConn fills it from cfg at PreRunE time.
	migrateCmd.PersistentFlags().StringVar(&urlConn, "url", "", "Database driver url (defaults to the configured Postgres connection)")
}

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
	cfg := configFrom(cmd)
	resolveURLConn(cfg)
	if urlConn == "" {
		return ErrURLNotSet
	}
	cmd.SilenceUsage = true
	if err := app.EnsureAdapter(cfg); err != nil {
		return err
	}
	sc := cfg.Adapter.ShowTable("public", "schema_migrations")
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
		db, err := app.PostgresDB(cfg)
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

// resolveURLConn fills urlConn from the configured Postgres connection when
// the --url flag was left unset. The flag itself is registered with an empty
// default (not a DSN built with the plaintext password), so `prestd migrate
// --help` never echoes the credential back to the terminal; this is where
// that default gets resolved instead, once flag parsing has already run.
func resolveURLConn(cfg *config.Prest) {
	if urlConn == "" {
		urlConn = driverURL(cfg)
	}
}

func driverURL(cfg *config.Prest) string {
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

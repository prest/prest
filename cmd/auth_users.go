package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	errPasswordNotTerminal = errors.New("stdin is not a terminal: pipe the password in and pass --password-stdin")
	errPasswordMismatch    = errors.New("passwords do not match")
)

// userCreator is the part of controllers.AuthHandler that create-user uses.
type userCreator interface {
	CreateUser(ctx context.Context, u controllers.NewUser) (auth.User, error)
}

// connectUserCreator connects the configured database and returns the auth
// handler, which knows the auth table, its columns and auth.encrypt.
func connectUserCreator(cfg *config.Prest) (userCreator, error) {
	if err := app.EnsureAdapter(cfg); err != nil {
		return nil, err
	}
	deps := controllers.NewDepsFromConfig(cfg)
	return controllers.NewAuthHandler(deps.Executor, deps.Dialect, deps.Auth), nil
}

// newAuthCmd builds the `auth` command group with its subcommands.
func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the users of the auth endpoint",
		Long:  "Manage the users that log in through the /auth endpoint",
	}
	cmd.AddCommand(newAuthCreateUserCmd(connectUserCreator))
	return cmd
}

// newAuthCreateUserCmd builds `auth create-user`, which stores the user
// through the creator connect returns.
func newAuthCreateUserCmd(connect func(*config.Prest) (userCreator, error)) *cobra.Command {
	var (
		name          string
		metadata      string
		passwordStdin bool
	)
	cmd := &cobra.Command{
		Use:   "create-user USERNAME",
		Short: "Create a user that can log in through /auth",
		Long: `Create a user in the auth table (auth.schema and auth.table) that can log in
through the /auth endpoint.

The password is hashed with the auth.encrypt algorithm. It is read from the
terminal without echo, or from stdin with --password-stdin. The username is
stored in lower case, the way /auth looks it up.

The auth table must exist. prestd creates it on startup when auth is enabled;
on Postgres "prestd migrate up auth" creates it too.`,
		Example: `  prestd auth create-user alice --name "Alice" --metadata '{"role":"admin"}'
  printf '%s' "$ALICE_PASSWORD" | prestd auth create-user alice --password-stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			password, err := readNewPassword(cmd, passwordStdin)
			if err != nil {
				return err
			}
			creator, err := connect(configFrom(cmd))
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			user, err := creator.CreateUser(cmd.Context(), controllers.NewUser{
				Name:     name,
				Username: args[0],
				Password: password,
				Metadata: metadata,
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created user %q (id %d)\n", user.Username, user.ID)
			return err
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "name", "", "Display name of the user")
	flags.StringVar(&metadata, "metadata", "", "JSON document stored in the metadata column")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read the password from stdin")
	return cmd
}

// readNewPassword reads the password from stdin when fromStdin is set,
// dropping one trailing newline. Otherwise it prompts on the terminal twice
// without echo and checks that both entries match.
func readNewPassword(cmd *cobra.Command, fromStdin bool) (string, error) {
	in := cmd.InOrStdin()
	if fromStdin {
		raw, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		password := strings.TrimSuffix(string(raw), "\n")
		return strings.TrimSuffix(password, "\r"), nil
	}
	f, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return "", errPasswordNotTerminal
	}
	first, err := promptPassword(cmd.ErrOrStderr(), f, "Password: ")
	if err != nil {
		return "", err
	}
	second, err := promptPassword(cmd.ErrOrStderr(), f, "Password (again): ")
	if err != nil {
		return "", err
	}
	if !bytes.Equal(first, second) {
		return "", errPasswordMismatch
	}
	return string(first), nil
}

func promptPassword(out io.Writer, f *os.File, prompt string) ([]byte, error) {
	fmt.Fprint(out, prompt)
	password, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(out)
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return password, nil
}

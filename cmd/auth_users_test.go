package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type fakeUserCreator struct {
	got  controllers.NewUser
	user auth.User
	err  error
}

func (f *fakeUserCreator) CreateUser(_ context.Context, u controllers.NewUser) (auth.User, error) {
	f.got = u
	return f.user, f.err
}

// runCreateUser runs `auth create-user` with args and stdin against creator
// and returns its stdout.
func runCreateUser(t *testing.T, creator userCreator, stdin string, args ...string) (string, error) {
	t.Helper()
	return runCreateUserWith(t, func(*config.Prest) (userCreator, error) { return creator, nil }, stdin, args...)
}

// runCreateUserWith is runCreateUser with the database connection replaced by
// connect.
func runCreateUserWith(t *testing.T, connect func(*config.Prest) (userCreator, error), stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newAuthCreateUserCmd(connect)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(withConfig(context.Background(), &config.Prest{}))
	return out.String(), err
}

func TestAuthCreateUser(t *testing.T) {
	t.Parallel()

	creator := &fakeUserCreator{user: auth.User{ID: 3, Username: "alice"}}

	out, err := runCreateUser(t, creator, "s3cret\n",
		"Alice", "--password-stdin", "--name", "Alice Doe", "--metadata", `{"role":"admin"}`)
	require.NoError(t, err)
	require.Equal(t, controllers.NewUser{
		Name:     "Alice Doe",
		Username: "Alice",
		Password: "s3cret",
		Metadata: `{"role":"admin"}`,
	}, creator.got)
	require.Equal(t, "created user \"alice\" (id 3)\n", out)
}

func TestAuthCreateUser_CreateError(t *testing.T) {
	t.Parallel()

	creator := &fakeUserCreator{err: controllers.ErrUserExists}

	_, err := runCreateUser(t, creator, "s3cret", "alice", "--password-stdin")
	require.ErrorIs(t, err, controllers.ErrUserExists)
}

func TestAuthCreateUser_ConnectError(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("connection refused")
	connect := func(*config.Prest) (userCreator, error) { return nil, dbErr }

	_, err := runCreateUserWith(t, connect, "s3cret", "alice", "--password-stdin")
	require.ErrorIs(t, err, dbErr)
	require.ErrorContains(t, err, "connect to database")
}

// Without --password-stdin the password is only read from a terminal, so a
// script that forgets the flag fails instead of creating a user.
func TestAuthCreateUser_StdinNotTerminal(t *testing.T) {
	t.Parallel()

	creator := &fakeUserCreator{}

	_, err := runCreateUser(t, creator, "s3cret\n", "alice")
	require.ErrorIs(t, err, errPasswordNotTerminal)
	require.Empty(t, creator.got.Username, "nothing is created without a password")
}

func TestAuthCreateUser_RequiresUsername(t *testing.T) {
	t.Parallel()

	_, err := runCreateUser(t, &fakeUserCreator{}, "s3cret", "--password-stdin")
	require.Error(t, err)
}

func TestReadNewPassword_Stdin(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"s3cret":       "s3cret",
		"s3cret\n":     "s3cret",
		"s3cret\r\n":   "s3cret",
		"two words\n":  "two words",
		"trailing \n":  "trailing ",
		"s3cret\n\n":   "s3cret\n",
		"":             "",
		"\n":           "",
		"pa\nss\nword": "pa\nss\nword",
	} {
		cmd := &cobra.Command{}
		cmd.SetIn(strings.NewReader(in))
		got, err := readNewPassword(cmd, true)
		require.NoError(t, err)
		require.Equal(t, want, got, "stdin %q", in)
	}
}

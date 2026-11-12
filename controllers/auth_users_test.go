package controllers

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// expectNoUser makes the username lookup of CreateUser find no row.
func expectNoUser(t *testing.T, ctrl *gomock.Controller, executor *mockgen.MockQueryExecutor, h *AuthHandler, username string) {
	t.Helper()
	sc := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().
		QueryCtx(gomock.Any(), mustQuery(t)(h.selectQueryByUsername()), username).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).Return(0, nil)
}

// captured matches any value and records it, so a test can check what was
// bound to the insert.
type captured struct{ value interface{} }

func (c *captured) Matches(x interface{}) bool { c.value = x; return true }
func (c *captured) String() string             { return "is captured" }

// expectInsert answers the insert of cols by CreateUser with the row the
// database returns, and hands the values bound to it to check.
func expectInsert(t *testing.T, ctrl *gomock.Controller, executor *mockgen.MockQueryExecutor, h *AuthHandler, cols []string, row auth.User, check func(params []interface{})) {
	t.Helper()
	params := make([]*captured, len(cols))
	matchers := make([]interface{}, len(cols))
	for i := range params {
		params[i] = &captured{}
		matchers[i] = params[i]
	}
	sc := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().
		InsertCtx(gomock.Any(), mustQuery(t)(h.insertUserQuery(cols)), matchers...).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		values := make([]interface{}, len(params))
		for i, p := range params {
			values[i] = p.value
		}
		check(values)
		*dest.(*auth.User) = row
		return 1, nil
	})
}

func TestAuthHandler_CreateUser_bcrypt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	expectNoUser(t, ctrl, executor, h, "alice")
	cols := []string{"username", "password", "name", "metadata"}
	expectInsert(t, ctrl, executor, h, cols, auth.User{ID: 4, Name: "Alice", Username: "alice"}, func(params []interface{}) {
		require.Equal(t, "alice", params[0])
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(params[1].(string)), []byte("s3cret")))
		require.Equal(t, "Alice", params[2])
		require.Equal(t, `{"role":"admin"}`, params[3])
	})

	user, err := h.CreateUser(context.Background(), NewUser{
		Name:     "Alice",
		Username: "alice",
		Password: "s3cret",
		Metadata: `{"role":"admin"}`,
	})
	require.NoError(t, err)
	require.Equal(t, auth.User{ID: 4, Name: "Alice", Username: "alice"}, user)
}

// The stored value must be what Login checks, for every auth.encrypt.
func TestAuthHandler_CreateUser_legacyDigest(t *testing.T) {
	t.Parallel()

	for algorithm, digest := range map[string]string{
		"MD5":  md5Hex("s3cret"),
		"sha1": fmt.Sprintf("%x", sha1.Sum([]byte("s3cret"))),
	} {
		t.Run(algorithm, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			executor := mockgen.NewMockQueryExecutor(ctrl)
			cfg := testAuthConfig()
			cfg.Encrypt = algorithm
			h := NewAuthHandler(executor, pgDialect{}, cfg)

			expectNoUser(t, ctrl, executor, h, "bob")
			// No name or metadata given: only the required columns are written.
			cols := []string{"username", "password"}
			expectInsert(t, ctrl, executor, h, cols, auth.User{ID: 5, Username: "bob"}, func(params []interface{}) {
				require.Equal(t, []interface{}{"bob", digest}, params)
			})

			_, err := h.CreateUser(context.Background(), NewUser{Username: "bob", Password: "s3cret"})
			require.NoError(t, err)
		})
	}
}

// Login lowercases the username it looks up, so a mixed-case username must
// be stored in lower case or the user could never log in.
func TestAuthHandler_CreateUser_lowercasesUsername(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())

	expectNoUser(t, ctrl, executor, h, "alice")
	expectInsert(t, ctrl, executor, h, []string{"username", "password"}, auth.User{ID: 6, Username: "alice"}, func(params []interface{}) {
		require.Equal(t, "alice", params[0])
	})

	_, err := h.CreateUser(context.Background(), NewUser{Username: " Alice ", Password: "s3cret"})
	require.NoError(t, err)
}

// A user created with bcrypt logs in through the same check /auth runs.
func TestAuthHandler_CreateUser_thenLogin(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	var stored string
	expectNoUser(t, ctrl, executor, h, "carol")
	expectInsert(t, ctrl, executor, h, []string{"username", "password"}, auth.User{ID: 7, Username: "carol"}, func(params []interface{}) {
		stored = params[1].(string)
	})
	_, err := h.CreateUser(context.Background(), NewUser{Username: "carol", Password: "pw"})
	require.NoError(t, err)

	sc := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().Query(mustQuery(t)(h.selectQueryByUsername()), "carol").Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		*dest.(*loginRow) = loginRow{ID: 7, Username: "carol", Password: stored}
		return 1, nil
	})
	user, err := h.basicPasswordCheck("carol", "pw")
	require.NoError(t, err)
	require.Equal(t, 7, user.ID)
}

func TestAuthHandler_CreateUser_exists(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())

	sc := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().
		QueryCtx(gomock.Any(), mustQuery(t)(h.selectQueryByUsername()), "alice").
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).Return(1, nil)

	_, err := h.CreateUser(context.Background(), NewUser{Username: "alice", Password: "s3cret"})
	require.ErrorIs(t, err, ErrUserExists)
	require.ErrorContains(t, err, "alice")
}

func TestAuthHandler_CreateUser_invalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		encrypt string
		user    NewUser
		want    error
	}{
		{"blank username", "MD5", NewUser{Username: "  ", Password: "pw"}, ErrEmptyUsername},
		{"empty password", "MD5", NewUser{Username: "alice"}, ErrEmptyPassword},
		{"metadata not JSON", "MD5", NewUser{Username: "alice", Password: "pw", Metadata: "{role:admin}"}, ErrInvalidMetadata},
		{"unknown algorithm", "PLAINTEXT", NewUser{Username: "alice", Password: "pw"}, ErrUnknownEncryptAlgorithm},
		{"password over bcrypt's 72 bytes", "bcrypt", NewUser{Username: "alice", Password: string(make([]byte, 73))}, bcrypt.ErrPasswordTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A nil executor proves nothing reaches the database.
			cfg := testAuthConfig()
			cfg.Encrypt = tc.encrypt
			h := NewAuthHandler(nil, pgDialect{}, cfg)
			_, err := h.CreateUser(context.Background(), tc.user)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAuthHandler_CreateUser_databaseErrors(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("relation \"public.prest_users\" does not exist")

	t.Run("lookup", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		executor := mockgen.NewMockQueryExecutor(ctrl)
		h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())
		sc := mockgen.NewMockScanner(ctrl)
		executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), "alice").Return(sc)
		sc.EXPECT().Err().Return(dbErr)

		_, err := h.CreateUser(context.Background(), NewUser{Username: "alice", Password: "pw"})
		require.ErrorIs(t, err, dbErr)
	})

	t.Run("insert", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		executor := mockgen.NewMockQueryExecutor(ctrl)
		h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())
		expectNoUser(t, ctrl, executor, h, "alice")
		sc := mockgen.NewMockScanner(ctrl)
		executor.EXPECT().InsertCtx(gomock.Any(), gomock.Any(), "alice", gomock.Any()).Return(sc)
		sc.EXPECT().Err().Return(dbErr)

		_, err := h.CreateUser(context.Background(), NewUser{Username: "alice", Password: "pw"})
		require.ErrorIs(t, err, dbErr)
	})
}

func TestAuthHandler_insertUserQuery(t *testing.T) {
	t.Parallel()

	cfg := testAuthConfig()
	cols := []string{"login", "secret_hash", "name", "metadata"}

	pg := mustQuery(t)(NewAuthHandler(nil, pgDialect{}, cfg).insertUserQuery(cols))
	require.Equal(t,
		`INSERT INTO "public"."prest_users" ("login", "secret_hash", "name", "metadata") VALUES ($1, $2, $3, $4)`, pg)

	my := mustQuery(t)(NewAuthHandler(nil, mysqlDialect{}, cfg).insertUserQuery(cols[:2]))
	require.Equal(t,
		"INSERT INTO `public`.`prest_users` (`login`, `secret_hash`) VALUES (?, ?)", my)

	_, err := NewAuthHandler(nil, nil, cfg).insertUserQuery(cols)
	require.ErrorIs(t, err, ErrAuthDialectMissing)

	_, err = NewAuthHandler(nil, pgDialect{}, cfg).insertUserQuery([]string{"username", "bad;column"})
	require.Error(t, err)
}

// auth.username and auth.password name the columns CreateUser writes.
func TestAuthHandler_CreateUser_configuredColumns(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	cfg := testAuthConfig()
	cfg.Username = "login"
	cfg.Password = "secret_hash"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	expectNoUser(t, ctrl, executor, h, "dave")
	expectInsert(t, ctrl, executor, h, []string{"login", "secret_hash", "name"}, auth.User{ID: 8, Name: "Dave", Username: "dave"}, func(params []interface{}) {
		require.Equal(t, []interface{}{"dave", md5Hex("s3cret"), "Dave"}, params)
	})

	_, err := h.CreateUser(context.Background(), NewUser{Name: "Dave", Username: "dave", Password: "s3cret"})
	require.NoError(t, err)
}

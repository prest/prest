package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/prest/prest/v2/controllers/auth"
)

var (
	// ErrUserExists is returned by CreateUser when the username is taken.
	ErrUserExists = errors.New("user already exists")
	// ErrEmptyUsername is returned by CreateUser for a blank username.
	ErrEmptyUsername = errors.New("username is required")
	// ErrEmptyPassword is returned by CreateUser for an empty password.
	ErrEmptyPassword = errors.New("password is required")
	// ErrInvalidMetadata is returned by CreateUser when the metadata is not
	// a JSON document.
	ErrInvalidMetadata = errors.New("metadata must be valid JSON")
)

// NewUser holds what CreateUser stores for a new user.
type NewUser struct {
	Username string
	Password string
	// Name is stored in the name column; empty leaves the column out.
	Name string
	// Metadata is a JSON document stored in the metadata column and returned
	// in the user_info of /auth responses; empty leaves the column out.
	Metadata string
}

// CreateUser adds a user to the auth table so it can log in through /auth.
//
// The username is stored in lower case, because Login lowercases the
// username it looks up. The password is hashed with the configured
// auth.encrypt algorithm, so the stored value is what Login checks against.
func (h *AuthHandler) CreateUser(ctx context.Context, u NewUser) (user auth.User, err error) {
	username := strings.ToLower(strings.TrimSpace(u.Username))
	if username == "" {
		return user, ErrEmptyUsername
	}
	if u.Password == "" {
		return user, ErrEmptyPassword
	}
	if u.Metadata != "" && !json.Valid([]byte(u.Metadata)) {
		return user, ErrInvalidMetadata
	}
	stored, err := h.storedPassword(u.Password)
	if err != nil {
		return user, err
	}
	exists, err := h.userExists(ctx, username)
	if err != nil {
		return user, err
	}
	if exists {
		return user, fmt.Errorf("%w: %s", ErrUserExists, username)
	}
	// name and metadata are only written when set, so a table that lacks
	// those optional columns still takes users.
	cols := []string{h.cfg.Username, h.cfg.Password}
	values := []any{username, stored}
	if u.Name != "" {
		cols = append(cols, "name")
		values = append(values, u.Name)
	}
	if u.Metadata != "" {
		cols = append(cols, "metadata")
		values = append(values, u.Metadata)
	}
	query, err := h.insertUserQuery(cols)
	if err != nil {
		return user, err
	}
	sc := h.executor.InsertCtx(ctx, query, values...)
	if err = sc.Err(); err != nil {
		return user, err
	}
	if _, err = sc.Scan(&user); err != nil {
		return user, err
	}
	return user, nil
}

// storedPassword returns the value Login compares a password against for
// the configured algorithm: a bcrypt hash, or a legacy hex digest.
func (h *AuthHandler) storedPassword(password string) (string, error) {
	switch strings.ToUpper(h.cfg.Encrypt) {
	case "BCRYPT":
		return HashPassword(password)
	case "MD5", "SHA1":
		return h.legacyDigest(password)
	default:
		return "", ErrUnknownEncryptAlgorithm
	}
}

func (h *AuthHandler) userExists(ctx context.Context, username string) (bool, error) {
	query, err := h.selectQueryByUsername()
	if err != nil {
		return false, err
	}
	sc := h.executor.QueryCtx(ctx, query, username)
	if err = sc.Err(); err != nil {
		return false, err
	}
	var row loginRow
	n, err := sc.Scan(&row)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// insertUserQuery builds an insert of cols into the auth table with the
// adapter's quoting and binds.
func (h *AuthHandler) insertUserQuery(cols []string) (string, error) {
	if h.dialect == nil {
		return "", ErrAuthDialectMissing
	}
	table, err := h.dialect.QuoteIdentifier(h.cfg.Schema + "." + h.cfg.Table)
	if err != nil {
		return "", err
	}
	quoted := make([]string, len(cols))
	binds := make([]string, len(cols))
	for i, col := range cols {
		if quoted[i], err = h.dialect.QuoteIdentifier(col); err != nil {
			return "", err
		}
		binds[i] = h.dialect.Placeholder(i + 1)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(quoted, ", "), strings.Join(binds, ", ")), nil
}

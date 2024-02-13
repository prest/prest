package controllers

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/structy/log"
	signer "gopkg.in/square/go-jose.v2"
	jwt "gopkg.in/square/go-jose.v2/jwt"

	"github.com/prest/prest/controllers/auth"
)

const unf = "user not found"

// Response representation
type Response struct {
	LoggedUser interface{} `json:"user_info"`
	Token      string      `json:"token"`
}

// RavensRequest representation
type RavensRequest struct {
	Type       string   `json:"type_of"`
	Subject    string   `json:"subject"`
	Recipients []string `json:"recipients"`
	Sender     string   `json:"sender"`
	SenderName string   `json:"sender_name"`
	Content    string   `json:"content"`
}

// Login representation of data received in authentication
type Login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Token generates a JWT token for the given user with the specified key.
// It uses the HS256 algorithm for signing the token.
// The token includes the user information, start time (NotBefore), and expiration time.
// The generated token is returned as a string.
// If an error occurs during token generation, it is returned along with an empty string.
//
// todo: add expiry time in configuration (in minute format, so we support the maximum need
// TODO: JWT any Algorithm support
func Token(u auth.User, key string) (t string, err error) {
	getToken := time.Now()
	expireToken := time.Now().Add(time.Hour * 6)

	sig, err := signer.NewSigner(
		signer.SigningKey{
			Algorithm: signer.HS256,
			Key:       []byte(key)},
		(&signer.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return
	}

	cl := auth.Claims{
		UserInfo:  u,
		NotBefore: jwt.NewNumericDate(getToken),
		Expiry:    jwt.NewNumericDate(expireToken),
	}
	return jwt.Signed(sig).Claims(cl).CompactSerialize()
}

// Auth handles the authentication logic based on the configured authentication type.
//
// The authentication type can be either "body" or "basic".
// If the authentication type is "body", it expects the login credentials to be provided in the request body as JSON.
// If the authentication type is "basic", it expects the login credentials to be provided in the request headers using HTTP Basic Authentication.
// It returns the logged-in user information and a token if the authentication is successful.
// If there is an error during the authentication process, it returns an HTTP error response.
//
// todo: add form support
func (c *Config) Auth(w http.ResponseWriter, r *http.Request) {
	log.Debugln("Authenticating user")
	login := Login{}
	switch c.server.AuthType {
	case "body":
		// to use body field authentication
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		//nolint
		dec.Decode(&login)
	case "basic":
		// to use http basic authentication
		var ok bool
		login.Username, login.Password, ok = r.BasicAuth()
		if !ok {
			log.Errorln(unf)
			JSONError(w, unf, http.StatusBadRequest)
			return
		}
	}

	loggedUser, err := c.basicPasswordCheck(r.Context(),
		strings.ToLower(login.Username), login.Password)
	if err != nil {
		log.Errorln(err)
		JSONError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	token, err := Token(loggedUser, c.server.JWTKey)
	if err != nil {
		log.Errorln(err)
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := Response{
		LoggedUser: loggedUser,
		Token:      token,
	}

	log.Debugln("User authenticated")
	JSONWrite(w, resp, http.StatusOK)
}

// basicPasswordCheck will check if the user and password are valid
//
// table name, fields (user and password) and encryption must be defined in
// the configuration file (toml) by default this endpoint will not be available,
// it is necessary to activate in the configuration file
func (c *Config) basicPasswordCheck(ctx context.Context, user, password string) (obj auth.User, err error) {
	sc := c.adapter.QueryCtx(ctx,
		c.getSelectQuery(), user, encrypt(c.server.AuthEncrypt, password))
	if sc.Err() != nil {
		err = sc.Err()
		return
	}
	n, err := sc.Scan(&obj)
	if err != nil {
		return
	}
	if n != 1 {
		err = fmt.Errorf(unf)
	}
	return
}

// getSelectQuery create the query to authenticate the user
// todo: fix how this password is queried/stored
func (c *Config) getSelectQuery() (query string) {
	return fmt.Sprintf(
		`SELECT * FROM %s.%s WHERE %s=$1 AND %s=$2 LIMIT 1`,
		c.server.AuthSchema, c.server.AuthTable,
		c.server.AuthUsername, c.server.AuthPassword)
}

// encrypt will apply the encryption algorithm to the password
func encrypt(encrypt, password string) (encrypted string) {
	switch encrypt {
	case "MD5":
		return fmt.Sprintf("%x", md5.Sum([]byte(password)))
	case "SHA1":
		return fmt.Sprintf("%x", sha1.Sum([]byte(password)))
	}
	return
}

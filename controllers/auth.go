package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prest/prest/controllers/auth"

	signer "gopkg.in/square/go-jose.v2"
	jwt "gopkg.in/square/go-jose.v2/jwt"
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

// Token for user
func (c *Config) Token(u auth.User) (t string, err error) {
	// add start time (NotBefore)
	getToken := time.Now()
	// add expiry time in configuration (in minute format, so we support the maximum need)
	expireToken := time.Now().Add(time.Hour * 6)

	// TODO: JWT any Algorithm support
	sig, err := signer.NewSigner(
		signer.SigningKey{
			Algorithm: signer.HS256,
			Key:       []byte(c.server.JWTKey)},
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

// Auth controller
func (c *Config) Auth(w http.ResponseWriter, r *http.Request) {
	login := Login{}
	switch c.server.AuthType {
	// TODO: form support
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
			http.Error(w, unf, http.StatusBadRequest)
			return
		}
	}

	loggedUser, err := c.basicPasswordCheck(strings.ToLower(login.Username), login.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	token, err := c.Token(loggedUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := Response{
		LoggedUser: loggedUser,
		Token:      token,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// basicPasswordCheck
func (c *Config) basicPasswordCheck(user, password string) (obj auth.User, err error) {
	/**
	table name, fields (user and password) and encryption must be defined in
	the configuration file (toml)
	by default this endpoint will not be available, it is necessary to activate
	in the configuration file
	*/
	// TODO: use Queryctx
	sc := c.adapter.Query(c.getSelectQuery(),
		user,
		encrypt(c.server.AuthEncrypt, password))
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

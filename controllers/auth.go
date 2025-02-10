package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers/auth"

	"gopkg.in/square/go-jose.v2"
	jwt "gopkg.in/square/go-jose.v2/jwt"
)

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
func Token(u auth.User) (t string, err error) {
	// add start time (NotBefore)
	getToken := time.Now()
	// add expiry time in configuration (in minute format, so we support the maximum need)
	expireToken := time.Now().Add(time.Hour * 6)

	// TODO: JWT any Algorithm support
	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.HS256,
			Key:       []byte(config.PrestConf.JWTKey)},
		(&jose.SignerOptions{}).WithType("JWT"))
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
func Auth(w http.ResponseWriter, r *http.Request) {
	login := Login{}
	switch config.PrestConf.AuthType {
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
			jsonError(w, unf, http.StatusBadRequest)
			return
		}
	}

	loggedUser, err := basicPasswordCheck(strings.ToLower(login.Username), login.Password)
	if err != nil {
		jsonError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	token, err := Token(loggedUser)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := Response{
		LoggedUser: loggedUser,
		Token:      token,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// basicPasswordCheck
func basicPasswordCheck(user, password string) (obj auth.User, err error) {
	/**
	table name, fields (user and password) and encryption must be defined in
	the configuration file (toml)
	by default this endpoint will not be available, it is necessary to activate
	in the configuration file
	*/
	sc := config.PrestConf.Adapter.Query(getSelectQuery(), user, encrypt(password))
	if sc.Err() != nil {
		err = sc.Err()
		return
	}
	n, err := sc.Scan(&obj)
	if err != nil {
		return
	}
	if n != 1 {
		err = ErrUserNotFound
	}

	return
}

// getSelectQuery create the query to authenticate the user
func getSelectQuery() (query string) {
	return fmt.Sprintf(
		`SELECT * FROM %s.%s WHERE %s=$1 AND %s=$2 LIMIT 1`,
		config.PrestConf.AuthSchema, config.PrestConf.AuthTable,
		config.PrestConf.AuthUsername, config.PrestConf.AuthPassword)
}

// encrypt will apply the encryption algorithm to the password
func encrypt(password string) (encrypted string) {
	switch config.PrestConf.AuthEncrypt {
	case "MD5":
		return fmt.Sprintf("%x", md5.Sum([]byte(password)))
	case "SHA1":
		return fmt.Sprintf("%x", sha1.Sum([]byte(password)))
	}
	return
}

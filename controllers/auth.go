package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers/auth"

	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
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

// AuthHandler serves the authentication endpoint.
type AuthHandler struct {
	executor adapters.QueryExecutor
	cfg      AuthConfig
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(executor adapters.QueryExecutor, cfg AuthConfig) *AuthHandler {
	return &AuthHandler{
		executor: executor,
		cfg:      cfg,
	}
}

// Login authenticates a user and returns a JWT.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	login := Login{}
	switch h.cfg.AuthType {
	case "body":
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		//nolint
		dec.Decode(&login)
	case "basic":
		var ok bool
		login.Username, login.Password, ok = r.BasicAuth()
		if !ok {
			jsonError(w, unf, http.StatusBadRequest)
			return
		}
	}

	loggedUser, err := h.basicPasswordCheck(strings.ToLower(login.Username), login.Password)
	if err != nil {
		jsonError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	token, err := h.token(loggedUser)
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

func (h *AuthHandler) token(u auth.User) (t string, err error) {
	getToken := time.Now()
	expireToken := time.Now().Add(time.Hour * 6)

	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.HS256,
			Key:       []byte(h.cfg.JWTKey)},
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

func (h *AuthHandler) basicPasswordCheck(user, password string) (obj auth.User, err error) {
	sc := h.executor.Query(h.selectQuery(), user, h.encrypt(password))
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

func (h *AuthHandler) selectQuery() (query string) {
	return fmt.Sprintf(
		`SELECT * FROM %s.%s WHERE %s=$1 AND %s=$2 LIMIT 1`,
		h.cfg.Schema, h.cfg.Table,
		h.cfg.Username, h.cfg.Password)
}

func (h *AuthHandler) encrypt(password string) (encrypted string) {
	switch h.cfg.Encrypt {
	case "MD5":
		return fmt.Sprintf("%x", md5.Sum([]byte(password)))
	case "SHA1":
		return fmt.Sprintf("%x", sha1.Sum([]byte(password)))
	}
	return
}

// Token creates a JWT for the given user using global config (legacy helper for tests).
func Token(u auth.User) (t string, err error) {
	h := NewAuthHandler(nil, AuthConfig{JWTKey: config.PrestConf.JWTKey})
	return h.token(u)
}

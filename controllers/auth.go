package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/prest/prest/config"
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

// AuthClaims JWT
type AuthClaims struct {
	jwt.StandardClaims
}

// Token for user
func Token(u User) (t string, err error) {
	// add expiry time in configuration (in minute format, so we support the maximum need)
	expireToken := time.Now().Add(time.Hour * 6).Unix()
	claims := AuthClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireToken,
			Id:        strconv.Itoa(u.ID),
			IssuedAt:  time.Now().Unix(),
			Issuer:    strconv.Itoa(u.CustomerID),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(config.PrestConf.JWTKey))
}

const unf = "user not found"

// Auth controller
func Auth(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok {
		http.Error(w, unf, http.StatusBadRequest)
		return
	}
	loggedUser, err := basicPasswordCheck(strings.ToLower(user), password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	token, err := Token(loggedUser)
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
func basicPasswordCheck(user, password string) (obj interface{}, err error) {
	/**
	table name, fields (user and password) and encryption must be defined in
	the configuration file (toml)
	by default this endpoint will not be available, it is necessary to activate
	in the configuration file
	*/
	query := `SELECT * FROM users WHERE user=$1 AND password=$2 LIMIT 1`
	sc := config.Get.DBAdapter.Query(query, user, password)
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
		return
	}
	return
}

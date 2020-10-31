package auth

import "github.com/dgrijalva/jwt-go"

// User logged in user representation
type User struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Username string      `json:"username"`
	Metadata interface{} `json:"metadata"`
}

// Claims JWT
type Claims struct {
	UserInfo User
	jwt.StandardClaims
}

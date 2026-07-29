package auth

import (
	"github.com/go-jose/go-jose/v4/jwt"
)

// User logged in user representation
type User struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Username string      `json:"username"`
	Metadata interface{} `json:"metadata"`
}

// Claims JWT
type Claims struct {
	UserInfo  User
	Expiry    *jwt.NumericDate `json:"exp,omitempty"`
	NotBefore *jwt.NumericDate `json:"nbf,omitempty"`
}

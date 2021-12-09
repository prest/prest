package auth

import (
	"context"

	jwt "github.com/form3tech-oss/jwt-go"
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
	UserInfo User
	jwt.StandardClaims
}

// Validate does nothing for this example.
func (c *Claims) Validate(ctx context.Context) error {
	/**
	if c.ShouldReject {
		return errors.New("should reject was set to true")
	}
	*/
	return nil
}

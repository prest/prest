package controllers

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUserNotFound = errors.New(unf)
	jsonErrorMsg    = `{"error":"%s"}`
)

const (
	unf = "user not found"
)

func jsonError(writer http.ResponseWriter, message string, status int) {
	http.Error(writer, fmt.Sprintf(jsonErrorMsg, message), status)
}

package postgres

import "errors"

var (
	ErrJoinInvalidNumberOfArgs = errors.New("invalid number of arguments in join statement")
	ErrInvalidIdentifier       = errors.New("invalid identifier")
)

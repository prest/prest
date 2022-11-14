package postgres

import "errors"

var (
	ErrJoinInvalidNumberOfArgs = errors.New("invalid number of arguments in join statement")
	ErrInvalidIdentifier       = errors.New("invalid identifier")
	ErrInvalidJoinClause       = errors.New("invalid join clause")
	ErrMustSelectOneField      = errors.New("you must select at least one field")
	ErrNoTableName             = errors.New("unable to find table name")
	ErrInvalidOperator         = errors.New("invalid operator")
	ErrInvalidGroupFn          = errors.New("invalid group function")
	// ErrBodyEmpty err throw when body is empty
	ErrBodyEmpty               = errors.New("body is empty")
	ErrEmptyOrInvalidSlice     = errors.New("empty or invalid slice")
)

package mysql

import "errors"

var (
	errInvalidIdentifier  = errors.New("invalid identifier")
	errInvalidOperator    = errors.New("invalid operator")
	errUnsupported        = errors.New("unsupported operator")
	errInvalidJoin        = errors.New("invalid join clause")
	errJoinArgs           = errors.New("invalid number of arguments in join statement")
	errMustSelectOneField = errors.New("you must select at least one field")
	errBodyEmpty          = errors.New("body is empty")
	errInvalidGroupFn     = errors.New("invalid group function")
)

package context

type Key int

const (
	_ Key = iota
	DBNameKey
	HTTPTimeoutKey
	UserInfoKey
	PrestConfigKey
	AdapterKey // Selected adapter for multi-database requests
)

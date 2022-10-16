package context

type Key int

const (
	_ Key = iota
	DBNameKey
	HTTPTimeoutKey
)

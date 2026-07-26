package contextkeys

type Key int

const (
	_ Key = iota
	DBNameKey
	HTTPTimeoutKey
	UserInfoKey
	PrestConfigKey
	AdapterKey          // Selected adapter for multi-database requests
	TxKey               // Transaction ID (string) bound to the current request (set by transaction middleware)
	TransactionManagerKey // *transactions.Manager for transaction handlers
)

package postgres

// Scanner interface to enable map pREST result to a struct
type Scanner interface {
	Scan(interface{}) (int, error)
	Bytes() []byte
	Err() error
}

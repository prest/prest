package adapters

// Scanner interface to enable map pREST result to a struct
type Scanner interface {
	// Scan copies the columns from the matched row into the value provided.
	// returns the number of columns copied into the interface (multiple copies due to Scan)
	Scan(interface{}) (int, error)

	// Bytes returns the bytes representation of the value
	Bytes() []byte

	// Err returns the error, if any, that was encountered during iteration.
	Err() error
}

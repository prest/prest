package adapters

// DatabaseRegistry tracks the active database connection name.
type DatabaseRegistry interface {
	SetDatabase(name string)
	GetDatabase() string
}

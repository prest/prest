package adapters

// DatabaseRegistry tracks the active database connection name.
type DatabaseRegistry interface {
	SetDatabase(name string)
	GetDatabase() string
	IsRegistered(alias string) bool
	PhysicalName(alias string) string
}

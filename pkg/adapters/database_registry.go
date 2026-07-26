package adapters

// DatabaseRegistry tracks the active database connection name.
type DatabaseRegistry interface {
	Aliases() []string
	SetDatabase(name string)
	GetDatabase() string
	IsRegistered(alias string) bool
	PhysicalName(alias string) string
}

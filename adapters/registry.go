package adapters

import (
	"errors"
	"sync"
)

// Registry maps database names/aliases to their corresponding adapters.
// This allows pREST to serve multiple SQL databases simultaneously,
// each with its own adapter (Postgres, TimescaleDB, MySQL, etc).
type Registry interface {
	// Register associates an adapter with a database alias.
	Register(alias string, adapter Adapter) error

	// Get retrieves an adapter by database alias.
	// Returns ErrAdapterNotFound if the alias is not registered.
	Get(alias string) (Adapter, error)

	// GetAll returns all registered adapter aliases.
	GetAll() []string

	// Aliases is a convenience alias for GetAll (for compatibility).
	Aliases() []string

	// IsRegistered reports whether an alias has an adapter.
	IsRegistered(alias string) bool
}

// ErrAdapterNotFound is returned when a requested adapter is not registered.
var ErrAdapterNotFound = errors.New("adapter not registered for database")

// SimpleRegistry is a thread-safe map-based adapter registry.
type SimpleRegistry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() Registry {
	return &SimpleRegistry{
		adapters: make(map[string]Adapter),
	}
}

// Register associates an adapter with a database alias.
func (r *SimpleRegistry) Register(alias string, adapter Adapter) error {
	if alias == "" {
		return errors.New("adapter alias cannot be empty")
	}
	if adapter == nil {
		return errors.New("adapter cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[alias] = adapter
	return nil
}

// Get retrieves an adapter by alias.
func (r *SimpleRegistry) Get(alias string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[alias]
	if !ok {
		return nil, ErrAdapterNotFound
	}
	return adapter, nil
}

// GetAll returns all registered aliases.
func (r *SimpleRegistry) GetAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	aliases := make([]string, 0, len(r.adapters))
	for alias := range r.adapters {
		aliases = append(aliases, alias)
	}
	return aliases
}

// Aliases is an alias for GetAll.
func (r *SimpleRegistry) Aliases() []string {
	return r.GetAll()
}

// IsRegistered reports whether an alias has an adapter.
func (r *SimpleRegistry) IsRegistered(alias string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.adapters[alias]
	return ok
}

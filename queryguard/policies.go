package queryguard

import "strings"

// Policies is the set of policies a guarded executor enforces: one default plus
// optional per-database overrides, keyed by database alias.
type Policies struct {
	// Default applies to every database without an override.
	Default Policy
	// Databases maps a database alias to the policy that replaces Default for it.
	Databases map[string]Policy
}

// IsZero reports whether no policy in the set enforces anything.
func (p Policies) IsZero() bool {
	if !p.Default.IsZero() {
		return false
	}
	for _, policy := range p.Databases {
		if !policy.IsZero() {
			return false
		}
	}
	return true
}

// For returns the policy that applies to a database alias. Aliases are matched
// case-insensitively; an unknown or empty alias falls back to Default.
func (p Policies) For(alias string) Policy {
	if alias == "" || len(p.Databases) == 0 {
		return p.Default
	}
	if policy, ok := p.Databases[alias]; ok {
		return policy
	}
	// Keys reaching here from configuration are already lower cased, but Policies
	// is also built directly by callers, so match the configured casing too.
	for key, policy := range p.Databases {
		if strings.EqualFold(key, alias) {
			return policy
		}
	}
	return p.Default
}

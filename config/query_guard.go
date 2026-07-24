package config

import (
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

// QueryGuardPolicy holds the execution-plan rules applied to read queries.
//
// It is the configuration form of queryguard.Policy; the composition root
// converts between the two so neither package depends on the other. Numeric
// limits are disabled when zero.
type QueryGuardPolicy struct {
	RejectSeqScan         bool     `mapstructure:"reject_seq_scan"`
	RejectParallelSeqScan bool     `mapstructure:"reject_parallel_seq_scan"`
	MaxCost               float64  `mapstructure:"max_cost"`
	MaxRows               float64  `mapstructure:"max_rows"`
	RequireIndexUsage     bool     `mapstructure:"require_index_usage"`
	MaxJoins              int      `mapstructure:"max_joins"`
	AllowTables           []string `mapstructure:"allow_tables"`
}

// QueryGuardConf holds the [query_guard] section.
//
// Default applies to every database; Databases holds per-alias overrides that
// start from Default and replace only the keys they set.
type QueryGuardConf struct {
	Enabled   bool
	Default   QueryGuardPolicy
	Databases map[string]QueryGuardPolicy
}

// PolicyFor returns the effective policy for a database alias. Aliases are
// matched case-insensitively because TOML section names are folded to lower case.
func (c QueryGuardConf) PolicyFor(alias string) QueryGuardPolicy {
	if policy, ok := c.Databases[strings.ToLower(alias)]; ok {
		return policy
	}
	return c.Default
}

// parseQueryGuardConfig reads the [query_guard] section. Env overrides use the
// PREST_QUERY_GUARD_* prefix (for example PREST_QUERY_GUARD_MAX_COST).
func parseQueryGuardConfig(v *viper.Viper, cfg *Prest) {
	g := &cfg.QueryGuard
	g.Enabled = v.GetBool("query_guard.enabled")
	g.Default = readQueryGuardPolicy(v, "query_guard", QueryGuardPolicy{})
	g.Databases = parseQueryGuardDatabases(v, g.Default)

	if g.Enabled && isEmptyQueryGuardPolicy(g.Default) && len(g.Databases) == 0 {
		slog.Warn("query_guard.enabled is set but no rule is configured; no query will be rejected")
	}
}

// readQueryGuardPolicy layers the keys present under prefix on top of base, so a
// per-database section only has to state what it changes.
func readQueryGuardPolicy(v *viper.Viper, prefix string, base QueryGuardPolicy) QueryGuardPolicy {
	policy := base
	if v.IsSet(prefix + ".reject_seq_scan") {
		policy.RejectSeqScan = v.GetBool(prefix + ".reject_seq_scan")
	}
	if v.IsSet(prefix + ".reject_parallel_seq_scan") {
		policy.RejectParallelSeqScan = v.GetBool(prefix + ".reject_parallel_seq_scan")
	}
	if v.IsSet(prefix + ".require_index_usage") {
		policy.RequireIndexUsage = v.GetBool(prefix + ".require_index_usage")
	}
	if v.IsSet(prefix + ".max_cost") {
		policy.MaxCost = nonNegativeFloat(v.GetFloat64(prefix+".max_cost"), prefix+".max_cost")
	}
	if v.IsSet(prefix + ".max_rows") {
		policy.MaxRows = nonNegativeFloat(v.GetFloat64(prefix+".max_rows"), prefix+".max_rows")
	}
	if v.IsSet(prefix + ".max_joins") {
		policy.MaxJoins = int(nonNegativeFloat(float64(v.GetInt(prefix+".max_joins")), prefix+".max_joins"))
	}
	if v.IsSet(prefix + ".allow_tables") {
		policy.AllowTables = v.GetStringSlice(prefix + ".allow_tables")
	}
	return policy
}

// parseQueryGuardDatabases reads [query_guard.databases.<alias>] overrides.
func parseQueryGuardDatabases(v *viper.Viper, base QueryGuardPolicy) map[string]QueryGuardPolicy {
	raw := v.GetStringMap("query_guard.databases")
	if len(raw) == 0 {
		return nil
	}
	policies := make(map[string]QueryGuardPolicy, len(raw))
	for alias := range raw {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			slog.Warn("query guard policy skipped: empty database alias")
			continue
		}
		policies[alias] = readQueryGuardPolicy(v, "query_guard.databases."+alias, base)
	}
	if len(policies) == 0 {
		return nil
	}
	return policies
}

// nonNegativeFloat disables a limit that was configured with a negative value,
// which would otherwise reject every query.
func nonNegativeFloat(value float64, key string) float64 {
	if value < 0 {
		slog.Warn("config key negative, limit disabled", "key", key, "value", value)
		return 0
	}
	return value
}

// isEmptyQueryGuardPolicy reports whether a policy would reject nothing.
func isEmptyQueryGuardPolicy(p QueryGuardPolicy) bool {
	return !p.RejectSeqScan &&
		!p.RejectParallelSeqScan &&
		!p.RequireIndexUsage &&
		p.MaxCost <= 0 &&
		p.MaxRows <= 0 &&
		p.MaxJoins <= 0
}

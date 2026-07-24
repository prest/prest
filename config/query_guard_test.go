package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// Query Guard is opt-in: without configuration nothing is enforced.
func TestParseQueryGuardDefaults(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))

	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)

	require.False(t, cfg.QueryGuard.Enabled)
	require.True(t, isEmptyQueryGuardPolicy(cfg.QueryGuard.Default))
	require.Nil(t, cfg.QueryGuard.Databases)
}

// Every rule from the [query_guard] section reaches the default policy.
func TestParseQueryGuardFullPolicy(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("query_guard.enabled", true)
	v.Set("query_guard.reject_seq_scan", true)
	v.Set("query_guard.reject_parallel_seq_scan", true)
	v.Set("query_guard.max_cost", 50000)
	v.Set("query_guard.max_rows", 100000)
	v.Set("query_guard.require_index_usage", true)
	v.Set("query_guard.max_joins", 3)
	v.Set("query_guard.allow_tables", []string{"lookup", "countries"})

	cfg := &Prest{}
	parseQueryGuardConfig(v, cfg)

	require.True(t, cfg.QueryGuard.Enabled)
	require.Equal(t, QueryGuardPolicy{
		RejectSeqScan:         true,
		RejectParallelSeqScan: true,
		MaxCost:               50000,
		MaxRows:               100000,
		RequireIndexUsage:     true,
		MaxJoins:              3,
		AllowTables:           []string{"lookup", "countries"},
	}, cfg.QueryGuard.Default)
}

// Env overrides use the PREST_QUERY_GUARD_* prefix.
func TestParseQueryGuardEnvOverride(t *testing.T) {
	t.Setenv("PREST_CONF", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("PREST_QUERY_GUARD_ENABLED", "true")
	t.Setenv("PREST_QUERY_GUARD_REJECT_SEQ_SCAN", "true")
	t.Setenv("PREST_QUERY_GUARD_MAX_COST", "1234.5")
	t.Setenv("PREST_QUERY_GUARD_MAX_JOINS", "2")

	v, configPath := viperCfg()
	cfg := &Prest{}
	Parse(v, cfg, configPath)

	require.True(t, cfg.QueryGuard.Enabled)
	require.True(t, cfg.QueryGuard.Default.RejectSeqScan)
	require.Equal(t, 1234.5, cfg.QueryGuard.Default.MaxCost)
	require.Equal(t, 2, cfg.QueryGuard.Default.MaxJoins)
}

// A per-database section starts from the global policy and replaces only the
// keys it sets, so operators do not have to restate every rule.
func TestParseQueryGuardPerDatabaseOverride(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("query_guard.enabled", true)
	v.Set("query_guard.reject_seq_scan", true)
	v.Set("query_guard.max_cost", 50000)
	v.Set("query_guard.databases.analytics.reject_seq_scan", false)
	v.Set("query_guard.databases.analytics.max_cost", 200000)

	cfg := &Prest{}
	parseQueryGuardConfig(v, cfg)

	analytics := cfg.QueryGuard.PolicyFor("analytics")
	require.False(t, analytics.RejectSeqScan)
	require.Equal(t, float64(200000), analytics.MaxCost)

	// Databases without an override keep the global policy.
	other := cfg.QueryGuard.PolicyFor("billing")
	require.True(t, other.RejectSeqScan)
	require.Equal(t, float64(50000), other.MaxCost)
}

// Alias lookup is case-insensitive: TOML section names are folded to lower case.
func TestQueryGuardPolicyForIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	conf := QueryGuardConf{
		Default:   QueryGuardPolicy{MaxCost: 10},
		Databases: map[string]QueryGuardPolicy{"analytics": {MaxCost: 99}},
	}

	require.Equal(t, float64(99), conf.PolicyFor("Analytics").MaxCost)
	require.Equal(t, float64(10), conf.PolicyFor("unknown").MaxCost)
}

// A negative limit would reject every query; it is treated as "no limit".
func TestParseQueryGuardRejectsNegativeLimits(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("query_guard.enabled", true)
	v.Set("query_guard.max_cost", -1)
	v.Set("query_guard.max_rows", -20)
	v.Set("query_guard.max_joins", -3)

	cfg := &Prest{}
	parseQueryGuardConfig(v, cfg)

	require.Zero(t, cfg.QueryGuard.Default.MaxCost)
	require.Zero(t, cfg.QueryGuard.Default.MaxRows)
	require.Zero(t, cfg.QueryGuard.Default.MaxJoins)
}

// Enabling the guard without any rule is accepted but leaves nothing enforced.
func TestParseQueryGuardEnabledWithoutRules(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("query_guard.enabled", true)

	cfg := &Prest{}
	parseQueryGuardConfig(v, cfg)

	require.True(t, cfg.QueryGuard.Enabled)
	require.True(t, isEmptyQueryGuardPolicy(cfg.QueryGuard.Default))
}

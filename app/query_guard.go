package app

import (
	"fmt"
	"log/slog"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/queryguard"
)

// applyQueryGuard decorates the table read executor with the configured Query
// Guard policies. Only statements generated from table CRUD requests are
// checked: auth, catalog and script SQL is written by pREST or by the operator,
// not by API clients.
//
// It fails when the guard is enabled but the adapter cannot produce execution
// plans. Starting anyway would serve an installation that believes it is
// protected while it is not.
func applyQueryGuard(cfg *config.Prest, deps *controllers.Deps) error {
	if cfg == nil || !cfg.QueryGuard.Enabled {
		return nil
	}
	policies := queryGuardPolicies(cfg.QueryGuard)
	// Decorate whatever is already designated as the table executor, so an
	// executor injected before this point keeps being the one that runs table
	// statements. Deps built by hand may leave it unset.
	inner := deps.TableExecutor
	if inner == nil {
		inner = deps.Executor
	}
	guarded, err := queryguard.NewExecutor(inner, policies)
	if err != nil {
		return fmt.Errorf("query guard: %w", err)
	}
	deps.TableExecutor = guarded
	slog.Info("query guard enabled", "databases_with_override", len(policies.Databases))
	return nil
}

// queryGuardPolicies converts the configuration form of the policies into the
// domain form, keeping config and queryguard independent of each other.
func queryGuardPolicies(conf config.QueryGuardConf) queryguard.Policies {
	policies := queryguard.Policies{Default: toQueryGuardPolicy(conf.Default)}
	if len(conf.Databases) == 0 {
		return policies
	}
	policies.Databases = make(map[string]queryguard.Policy, len(conf.Databases))
	for alias, policy := range conf.Databases {
		policies.Databases[alias] = toQueryGuardPolicy(policy)
	}
	return policies
}

// toQueryGuardPolicy maps one configured policy onto its domain counterpart.
func toQueryGuardPolicy(p config.QueryGuardPolicy) queryguard.Policy {
	return queryguard.Policy{
		RejectSeqScan:         p.RejectSeqScan,
		RejectParallelSeqScan: p.RejectParallelSeqScan,
		MaxCost:               p.MaxCost,
		MaxRows:               p.MaxRows,
		RequireIndexUsage:     p.RequireIndexUsage,
		MaxJoins:              p.MaxJoins,
		AllowTables:           p.AllowTables,
	}
}

package app

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/adapters/queryplan"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/queryguard"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// plannerAdapter is a mock adapter that can also explain statements, matching
// what the postgres adapter offers in production.
type plannerAdapter struct {
	*mockgen.MockAdapter
}

func (plannerAdapter) Explain(string, ...interface{}) (*queryplan.Node, error) {
	return &queryplan.Node{}, nil
}

func (plannerAdapter) ExplainCtx(context.Context, string, ...interface{}) (*queryplan.Node, error) {
	return &queryplan.Node{}, nil
}

func guardEnabledConf() *config.Prest {
	return &config.Prest{
		QueryGuard: config.QueryGuardConf{
			Enabled: true,
			Default: config.QueryGuardPolicy{RejectSeqScan: true},
		},
	}
}

// With the guard disabled the executor is left alone, so nothing pays for an
// EXPLAIN round trip it did not ask for.
func TestApplyQueryGuardDisabled(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := plannerAdapter{mockgen.NewMockAdapter(ctrl)}
	deps := controllers.Deps{Executor: executor, TableExecutor: executor}

	require.NoError(t, applyQueryGuard(&config.Prest{}, &deps))
	require.Same(t, executor.MockAdapter, deps.TableExecutor.(plannerAdapter).MockAdapter)
	require.NoError(t, applyQueryGuard(nil, &deps))
}

// Enabling the guard replaces only the table executor: auth, catalog and script
// statements keep running through the undecorated executor.
func TestApplyQueryGuardEnabled(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	general := plannerAdapter{mockgen.NewMockAdapter(ctrl)}
	table := plannerAdapter{mockgen.NewMockAdapter(ctrl)}
	deps := controllers.Deps{Executor: general, TableExecutor: table}

	require.NoError(t, applyQueryGuard(guardEnabledConf(), &deps))

	// Only the table executor is decorated, and it decorates the executor that was
	// already designated for table statements, not the general one.
	require.IsType(t, &queryguard.Executor{}, deps.TableExecutor)
	guarded := deps.TableExecutor.(*queryguard.Executor)
	require.Same(t, table.MockAdapter, guarded.QueryExecutor.(plannerAdapter).MockAdapter)
	require.Same(t, general.MockAdapter, deps.Executor.(plannerAdapter).MockAdapter)
}

// Deps built without a table executor fall back to the general one, so callers
// that never set the field keep working.
func TestApplyQueryGuardFallsBackToExecutor(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	general := plannerAdapter{mockgen.NewMockAdapter(ctrl)}
	deps := controllers.Deps{Executor: general}

	require.NoError(t, applyQueryGuard(guardEnabledConf(), &deps))

	guarded := deps.TableExecutor.(*queryguard.Executor)
	require.Same(t, general.MockAdapter, guarded.QueryExecutor.(plannerAdapter).MockAdapter)
}

// An adapter that cannot explain statements cannot enforce a policy: startup
// fails rather than serving an installation that believes it is protected.
func TestApplyQueryGuardWithoutPlannerFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	deps := controllers.Deps{Executor: mockgen.NewMockAdapter(ctrl)}

	err := applyQueryGuard(guardEnabledConf(), &deps)
	require.ErrorIs(t, err, queryguard.ErrPlannerUnsupported)
	require.Contains(t, err.Error(), "query guard")
}

// Every configured rule reaches the domain policy, including per-database
// overrides, so nothing is silently dropped in translation.
func TestQueryGuardPolicies(t *testing.T) {
	t.Parallel()

	conf := config.QueryGuardConf{
		Enabled: true,
		Default: config.QueryGuardPolicy{
			RejectSeqScan:         true,
			RejectParallelSeqScan: true,
			MaxCost:               50000,
			MaxRows:               100000,
			RequireIndexUsage:     true,
			MaxJoins:              3,
			AllowTables:           []string{"lookup", "countries"},
		},
		Databases: map[string]config.QueryGuardPolicy{
			"analytics": {MaxCost: 200000},
		},
	}

	policies := queryGuardPolicies(conf)

	require.Equal(t, queryguard.Policy{
		RejectSeqScan:         true,
		RejectParallelSeqScan: true,
		MaxCost:               50000,
		MaxRows:               100000,
		RequireIndexUsage:     true,
		MaxJoins:              3,
		AllowTables:           []string{"lookup", "countries"},
	}, policies.Default)
	require.Equal(t, float64(200000), policies.For("analytics").MaxCost)
	require.False(t, policies.For("analytics").RejectSeqScan)
}

// Without overrides the policy set carries only the default.
func TestQueryGuardPoliciesWithoutOverrides(t *testing.T) {
	t.Parallel()

	policies := queryGuardPolicies(config.QueryGuardConf{
		Default: config.QueryGuardPolicy{MaxRows: 10},
	})

	require.Nil(t, policies.Databases)
	require.Equal(t, float64(10), policies.For("anything").MaxRows)
}

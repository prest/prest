package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// These tests run against the prestd instance configured by
// testdata/prest_query_guard.toml: Query Guard enabled with reject_seq_scan and
// allow_tables = ["test2"]. Every integration table is unindexed, so reads are
// planned as sequential scans; only allow-listed tables get through.

func TestQueryGuard_RejectsSeqScan(t *testing.T) {
	base := helpers.QueryGuardServerURL(t)

	// Read a table that is not allow-listed.
	// Expected: 422 with the rule and the reason, since the plan is a Seq Scan.
	// Why: this is the whole point of the feature — refuse the query before the
	// database runs it, and tell the client which rule refused it.
	testutils.DoRequest(t,
		base+"/prest-test/public/test",
		nil, http.MethodGet,
		http.StatusUnprocessableEntity,
		"TestQueryGuard_RejectsSeqScan",
		`"error":"query rejected by Query Guard"`,
		`"rule":"reject_seq_scan"`,
		"Sequential Scan detected on table 'test'",
	)
}

func TestQueryGuard_AllowTablesExemption(t *testing.T) {
	base := helpers.QueryGuardServerURL(t)

	// Read a table listed in allow_tables, planned as the same Seq Scan.
	// Expected: 200, because the table opted out of the scan rules.
	// Why: small lookup tables must stay usable when a full scan is cheaper than
	// an index; without the exemption the guard would be unusable in practice.
	testutils.DoRequest(t,
		base+"/prest-test/public/test2",
		nil, http.MethodGet,
		http.StatusOK,
		"TestQueryGuard_AllowTablesExemption",
	)
}

func TestQueryGuard_CountIsGuardedToo(t *testing.T) {
	base := helpers.QueryGuardServerURL(t)

	// Count rows of a non allow-listed table (_count runs a separate statement).
	// Expected: 422 — counting scans the same table as reading it.
	// Why: a client must not be able to bypass the policy by asking for a count.
	testutils.DoRequest(t,
		base+"/prest-test/public/test?_count=*",
		nil, http.MethodGet,
		http.StatusUnprocessableEntity,
		"TestQueryGuard_CountIsGuardedToo",
		`"rule":"reject_seq_scan"`,
	)
}

func TestQueryGuard_WritesAreNotGuarded(t *testing.T) {
	base := helpers.QueryGuardServerURL(t)

	// Insert into a table whose reads are refused by the policy.
	// Expected: 201 — writes are never planned, so the policy does not apply.
	// Why: the guard protects read load; blocking writes would be a regression
	// for every deployment that enables it.
	testutils.DoRequest(t,
		base+"/prest-test/public/test",
		map[string]any{"name": "query-guard"},
		http.MethodPost,
		http.StatusCreated,
		"TestQueryGuard_WritesAreNotGuarded",
	)
}

func TestQueryGuard_DisabledServerIsUnaffected(t *testing.T) {
	base := helpers.ServerURL(t)

	// Same read on the default prestd instance, which has no Query Guard.
	// Expected: 200 — the feature is opt-in and must not change existing setups.
	testutils.DoRequest(t,
		base+"/prest-test/public/test",
		nil, http.MethodGet,
		http.StatusOK,
		"TestQueryGuard_DisabledServerIsUnaffected",
	)
}

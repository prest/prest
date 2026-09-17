package adapters_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/adapters"
)

// legacyQueryBuilder is a frozen snapshot of RequestQueryBuilder as an
// out-of-tree adapter implements it. Its only job is to fail compilation if a
// port method's signature changes.
//
// DO NOT "fix" this file to match a changed interface. If it stops compiling,
// the interface change is the bug: it breaks every adapter outside this repo,
// at compile time, with no deprecation path. Rework the change so the port
// stays as it is -- see the adapter-compatibility section of
// .cursor/rules/hexagonal-architecture.mdc.
//
// The in-repo doubles (adapters/mock, adapters/mockgen) cannot catch this:
// mockgen regenerates from whatever the interface currently says, so it always
// compiles. This file is the contract.
type legacyQueryBuilder struct{}

func (legacyQueryBuilder) WhereByRequest(r *http.Request, initialPlaceholderID int) (string, []interface{}, error) {
	return "", nil, nil
}
func (legacyQueryBuilder) DistinctClause(r *http.Request) (string, error)     { return "", nil }
func (legacyQueryBuilder) OrderByRequest(r *http.Request) (string, error)     { return "", nil }
func (legacyQueryBuilder) PaginateIfPossible(r *http.Request) (string, error) { return "", nil }
func (legacyQueryBuilder) JoinByRequest(r *http.Request) ([]string, error)    { return nil, nil }
func (legacyQueryBuilder) GroupByClause(r *http.Request) string               { return "" }
func (legacyQueryBuilder) TimeBucketClause(r *http.Request) (string, error)   { return "", nil }
func (legacyQueryBuilder) CountByRequest(req *http.Request) (string, error)   { return "", nil }
func (legacyQueryBuilder) ReturningByRequest(r *http.Request) (string, error) { return "", nil }
func (legacyQueryBuilder) SetByRequest(r *http.Request, initialPlaceholderID int) (string, []interface{}, error) {
	return "", nil, nil
}
func (legacyQueryBuilder) ParseInsertRequest(r *http.Request) (string, string, []interface{}, error) {
	return "", "", nil, nil
}
func (legacyQueryBuilder) ParseBatchInsertRequest(r *http.Request) (string, string, []interface{}, error) {
	return "", "", nil, nil
}

// The assertion is the test: this line is what breaks on a port change.
var _ adapters.RequestQueryBuilder = legacyQueryBuilder{}

func TestRequestQueryBuilderStaysCompatible(t *testing.T) {
	t.Parallel()

	// Reaching this line at all means the assertion above compiled, which is
	// the whole point. Calling through the interface keeps the vars used and
	// documents that the method set is exercised, not just declared.
	var builder adapters.RequestQueryBuilder = legacyQueryBuilder{}
	joins, err := builder.JoinByRequest(&http.Request{})
	if err != nil || joins != nil {
		t.Fatalf("frozen stub should be inert, got %v / %v", joins, err)
	}
}

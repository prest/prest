package adapters

import "net/http"

// RequestQueryBuilder parses HTTP request query parameters into SQL fragments.
type RequestQueryBuilder interface {
	WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error)
	DistinctClause(r *http.Request) (distinctQuery string, err error)
	OrderByRequest(r *http.Request) (values string, err error)
	PaginateIfPossible(r *http.Request) (paginatedQuery string, err error)
	JoinByRequest(r *http.Request) (values []string, err error)
	GroupByClause(r *http.Request) (groupBySQL string)
	TimeBucketClause(r *http.Request) (groupBySQL string, err error)
	CountByRequest(req *http.Request) (countQuery string, err error)
	ReturningByRequest(r *http.Request) (returningSyntax string, err error)
	SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error)
	ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseBatchInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
}

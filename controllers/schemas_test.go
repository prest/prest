package controllers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
)

func Test_GetSchemas(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		description string

		whereBySyntax string
		whereByValues []interface{}
		whereByError  error

		wantDistinct         bool
		schemaClauseSQL      string
		schemaClauseHasCount bool
		distinctClause       string
		distinctError        error

		wantOrderBy bool
		orderBy     string
		orderByErr  error

		wantSchemaOrderBy bool
		schemaOrderBy     string

		wantPaginateIfPossible bool
		paginateIfPossible     string
		paginateIfPossibleErr  error

		wantQuery bool
		queryErr  error

		wantQueryResp bool
		queryResp     string

		wantContains   string
		wantStatusCode int
	}{
		{
			description: "Get schemas where by error",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  errors.New("where by error"),

			wantContains:   "where by error",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			description: "Get schemas distinct error",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  nil,

			wantDistinct:         true,
			schemaClauseSQL:      "",
			schemaClauseHasCount: false,
			distinctClause:       "",
			distinctError:        errors.New("distinct error"),

			wantContains:   "distinct error",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			description: "Get schemas order by error",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  nil,

			wantDistinct:         true,
			schemaClauseSQL:      "",
			schemaClauseHasCount: false,
			distinctClause:       "",
			distinctError:        nil,

			wantOrderBy: true,
			orderBy:     "",
			orderByErr:  errors.New("order by error"),

			wantContains:   "order by error",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			description: "Get schemas paginate if possible error",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  nil,

			wantDistinct:         true,
			schemaClauseSQL:      "",
			schemaClauseHasCount: false,
			distinctClause:       "",
			distinctError:        nil,

			wantOrderBy: true,
			orderBy:     "",
			orderByErr:  nil,

			wantSchemaOrderBy: true,
			schemaOrderBy:     "",

			wantPaginateIfPossible: true,
			paginateIfPossible:     "",
			paginateIfPossibleErr:  errors.New("paginate if possible error"),

			wantContains:   "paginate if possible error",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			description: "Get schemas query error",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  nil,

			wantDistinct:         true,
			schemaClauseSQL:      "",
			schemaClauseHasCount: false,
			distinctClause:       "",
			distinctError:        nil,

			wantOrderBy: true,
			orderBy:     "",
			orderByErr:  nil,

			wantSchemaOrderBy: true,
			schemaOrderBy:     "",

			wantPaginateIfPossible: true,
			paginateIfPossible:     "",
			paginateIfPossibleErr:  nil,

			wantQuery: true,
			queryErr:  errors.New("query error"),

			wantContains:   "query error",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			description: "Get schemas OK",

			whereBySyntax: "",
			whereByValues: nil,
			whereByError:  nil,

			wantDistinct:         true,
			schemaClauseSQL:      "",
			schemaClauseHasCount: false,
			distinctClause:       "",
			distinctError:        nil,

			wantOrderBy: true,
			orderBy:     "",
			orderByErr:  nil,

			wantSchemaOrderBy: true,
			schemaOrderBy:     "",

			wantPaginateIfPossible: true,
			paginateIfPossible:     "",
			paginateIfPossibleErr:  nil,

			wantQuery: true,
			queryErr:  nil,

			wantQueryResp: true,
			queryResp:     "[{\"schema_name\": \"public\"}]",

			wantContains:   "public",
			wantStatusCode: http.StatusOK,
		},
		// todo: pass these to the adapter tests
		// {"Get schemas without custom where clause", "/schemas", "GET", http.StatusOK, "[{\"schema_name\": \"information_schema\"}, {\"schema_name\": \"pg_catalog\"}, {\"schema_name\": \"pg_toast\"}, {\"schema_name\": \"public\"}]"},
		// {"Get schemas with custom where clause", "/schemas?schema_name=$eq.public", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		// {"Get schemas with custom order clause", "/schemas?schema_name=$eq.public&_order=schema_name", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		// {"Get schemas with custom order invalid clause", "/schemas?schema_name=$eq.public&_order=$eq.schema_name", "GET", http.StatusBadRequest, "invalid identifier\n"},
		// {"Get schemas with custom where clause and pagination", "/schemas?schema_name=$eq.public&_page=1&_page_size=20", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		// {"Get schemas with COUNT clause", "/schemas?_count=*", "GET", http.StatusOK, "[{\"count\": 4}]"},
		// {"Get schemas with custom where invalid clause", "/schemas?0schema_name=$eq.public", "GET", http.StatusBadRequest, "0schema_name: invalid identifier\n"},
		// {"Get schemas with noexistent column", "/schemas?schematame=$eq.test", "GET", http.StatusBadRequest, "pq: column \"schematame\" does not exist\n"},
		// {"Get schemas with distinct clause", "/schemas?schema_name=$eq.public&_distinct=true", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			adapter := mockgen.NewMockAdapter(ctrl)

			ctrl2 := gomock.NewController(t)
			sc := mockgen.NewMockScanner(ctrl2)

			adapter.EXPECT().WhereByRequest(gomock.Any(), 1).
				Return(tc.whereBySyntax, tc.whereByValues, tc.whereByError)

			if tc.wantDistinct {
				adapter.EXPECT().SchemaClause(gomock.Any()).
					Return(tc.schemaClauseSQL, tc.schemaClauseHasCount)

				adapter.EXPECT().DistinctClause(gomock.Any()).
					Return(tc.distinctClause, tc.distinctError)
			}

			if tc.wantOrderBy {
				adapter.EXPECT().OrderByRequest(gomock.Any()).
					Return(tc.orderBy, tc.orderByErr)
			}

			if tc.wantSchemaOrderBy {
				adapter.EXPECT().SchemaOrderBy(tc.orderBy, tc.schemaClauseHasCount).
					Return(tc.schemaOrderBy)
			}

			if tc.wantPaginateIfPossible {
				adapter.EXPECT().PaginateIfPossible(gomock.Any()).
					Return(tc.paginateIfPossible, tc.paginateIfPossibleErr)
			}

			if tc.wantQueryResp {
				sc.EXPECT().Bytes().
					Return([]byte(tc.queryResp))
			}

			if tc.wantQuery {
				sc.EXPECT().Err().
					Return(tc.queryErr)

				adapter.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), tc.whereByValues...).
					Return(sc)
			}

			h := Config{adapter: adapter}

			req := httptest.NewRequest(http.MethodGet, "localhost:8080", nil)

			recorder := httptest.NewRecorder()

			h.GetSchemas(recorder, req)

			resp := recorder.Result()
			require.Equal(t, tc.wantStatusCode, resp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(data), tc.wantContains)
		})
	}
}

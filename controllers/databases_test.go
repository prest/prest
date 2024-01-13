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

var (
	dbErr = errors.New("random error")
)

func Test_GetDatabases(t *testing.T) {
	t.Parallel()
	var testCases = []struct {
		description     string
		wantRespContain string
		wantStatus      int
		wantParams      bool
		params          map[string]string

		whereByRequestSyntaxResp string
		whereByRequestValuesResp []interface{}
		whereByRequestErrResp    error

		wantDistinct       bool
		databaseWhereResp  string
		databaseClauseResp string
		hasCount           bool
		distinctClauseResp string
		distinctClauseErr  error

		wantOrderBy     bool
		wantOrderByErr  error
		wantOrderByResp string

		wantDatabaseOrderByResp string

		wantPaginate     bool
		wantPaginateErr  error
		wantPaginateResp string

		wantQuery     bool
		wantQueryErr  error
		wantQueryResp string
	}{
		{
			description:     "Get databases without custom where clause with error",
			wantStatus:      http.StatusBadRequest,
			wantRespContain: dbErr.Error(),

			whereByRequestSyntaxResp: "",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    dbErr,
		},
		{
			description:     "Get databases with distinct clause error",
			wantStatus:      http.StatusBadRequest,
			wantRespContain: dbErr.Error(),

			whereByRequestSyntaxResp: "syntax",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    nil,

			wantDistinct:       true,
			databaseWhereResp:  "where",
			databaseClauseResp: "",
			hasCount:           false,
			distinctClauseResp: "",
			distinctClauseErr:  dbErr,
		},
		{
			description:     "Get databases with order by request error",
			wantStatus:      http.StatusBadRequest,
			wantRespContain: dbErr.Error(),

			whereByRequestSyntaxResp: "syntax",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    nil,

			wantDistinct:       true,
			databaseWhereResp:  "where",
			databaseClauseResp: "",
			hasCount:           false,
			distinctClauseResp: "",
			distinctClauseErr:  nil,

			wantOrderBy:     true,
			wantOrderByErr:  dbErr,
			wantOrderByResp: "",
		},
		{
			description:     "Get databases with paginate error",
			wantStatus:      http.StatusBadRequest,
			wantRespContain: dbErr.Error(),

			whereByRequestSyntaxResp: "syntax",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    nil,

			wantDistinct:       true,
			databaseWhereResp:  "where",
			databaseClauseResp: "",
			hasCount:           false,
			distinctClauseResp: "",
			distinctClauseErr:  nil,

			wantOrderBy:     true,
			wantOrderByErr:  nil,
			wantOrderByResp: "",

			wantPaginate:            true,
			wantPaginateErr:         dbErr,
			wantPaginateResp:        "",
			wantDatabaseOrderByResp: "",
		},
		{
			description:     "Get databases with query error",
			wantStatus:      http.StatusBadRequest,
			wantRespContain: dbErr.Error(),

			whereByRequestSyntaxResp: "syntax",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    nil,

			wantDistinct:       true,
			databaseWhereResp:  "where",
			databaseClauseResp: "",
			hasCount:           false,
			distinctClauseResp: "",
			distinctClauseErr:  nil,

			wantOrderBy:     true,
			wantOrderByErr:  nil,
			wantOrderByResp: "",

			wantPaginate:            true,
			wantPaginateErr:         nil,
			wantPaginateResp:        "",
			wantDatabaseOrderByResp: "",

			wantQuery:     true,
			wantQueryErr:  dbErr,
			wantQueryResp: "",
		},
		{
			description:     "Get databases happy path",
			wantStatus:      http.StatusOK,
			wantRespContain: "test",

			whereByRequestSyntaxResp: "syntax",
			whereByRequestValuesResp: nil,
			whereByRequestErrResp:    nil,

			wantDistinct:       true,
			databaseWhereResp:  "where",
			databaseClauseResp: "",
			hasCount:           false,
			distinctClauseResp: "",
			distinctClauseErr:  nil,

			wantOrderBy:     true,
			wantOrderByErr:  nil,
			wantOrderByResp: "",

			wantPaginate:            true,
			wantPaginateErr:         nil,
			wantPaginateResp:        "",
			wantDatabaseOrderByResp: "",

			wantQuery:     true,
			wantQueryErr:  nil,
			wantQueryResp: `{"test": "test"}`,
		},
		// todo: add these to integration tests
		// {"Get databases without custom where clause", "/databases", "GET", http.StatusOK},
		// {"Get databases with custom where clause", "/databases?datname=$eq.prest", "GET", http.StatusOK},
		// {"Get databases with custom order clause", "/databases?_order=datname", "GET", http.StatusOK},
		// {"Get databases with custom order invalid clause", "/databases?_order=$eq.prest", "GET", http.StatusBadRequest},
		// {"Get databases with custom where clause and pagination", "/databases?datname=$eq.prest&_page=1&_page_size=20", "GET", http.StatusOK},
		// {"Get databases with COUNT clause", "/databases?_count=*", "GET", http.StatusOK},
		// {"Get databases with custom where invalid clause", "/databases?0datname=prest", "GET", http.StatusBadRequest},
		// {"Get databases with custom where and pagination invalid", "/databases?datname=$eq.prest&_page=A", "GET", http.StatusBadRequest},
		// {"Get databases with noexistent column", "/databases?datatata=$eq.test", "GET", http.StatusBadRequest},
		// {"Get databases with distinct", "/databases?_distinct=true", "GET", http.StatusOK},
		// {"Get databases with invalid distinct", "/databases?_distinct", "GET", http.StatusOK},
	}

	// todo: fix this test
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			t.Log(tc.description)

			ctrl := gomock.NewController(t)
			adapter := mockgen.NewMockAdapter(ctrl)

			ctrl2 := gomock.NewController(t)
			adapter2 := mockgen.NewMockScanner(ctrl2)

			adapter.EXPECT().WhereByRequest(
				gomock.Any(), gomock.Any()).
				Return(tc.whereByRequestSyntaxResp,
					tc.whereByRequestValuesResp, tc.whereByRequestErrResp)

			if tc.wantDistinct {
				adapter.EXPECT().DatabaseWhere(gomock.Any()).
					Return(tc.databaseWhereResp)

				adapter.EXPECT().DatabaseClause(gomock.Any()).
					Return(tc.databaseClauseResp, tc.hasCount)

				adapter.EXPECT().DistinctClause(gomock.Any()).
					Return(tc.distinctClauseResp, tc.distinctClauseErr)
			}

			if tc.wantOrderBy {
				adapter.EXPECT().OrderByRequest(gomock.Any()).
					Return(tc.wantOrderByResp, tc.wantOrderByErr)
			}

			if tc.wantPaginate {
				adapter.EXPECT().DatabaseOrderBy(gomock.Any(), gomock.Any()).
					Return(tc.wantDatabaseOrderByResp)
				adapter.EXPECT().PaginateIfPossible(gomock.Any()).
					Return(tc.wantPaginateResp, tc.wantPaginateErr)
			}

			if tc.wantQuery {
				adapter.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(adapter2)

				adapter2.EXPECT().Err().Return(tc.wantQueryErr)

				if tc.wantQueryErr == nil {
					adapter2.EXPECT().Bytes().Return([]byte(tc.wantQueryResp))
				}
			}

			cfg := *defaultConfig

			h := Config{
				server:  &cfg,
				adapter: adapter,
			}

			req := httptest.NewRequest(http.MethodGet, "localhost:8080", nil)

			recorder := httptest.NewRecorder()

			h.GetDatabases(recorder, req)
			resp := recorder.Result()
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(data), tc.wantRespContain)
		})
	}
}

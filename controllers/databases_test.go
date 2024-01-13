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
		description string

		whereByRequestSyntaxResp string
		whereByRequestValuesResp []interface{}
		whereByRequestErrResp    error

		wantParams bool
		params     map[string]string

		wantDistinct      bool
		databaseWhereResp string

		databaseClauseResp string
		hasCount           bool

		distinctClauseResp string
		distinctClauseErr  error

		wantRespContain string
		wantStatus      int
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
			_ = adapter2

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

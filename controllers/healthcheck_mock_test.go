package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/adapters/mock"
	"github.com/prest/prest/testutils"
)

type mockedDbConn struct {
	mock.Mock
}

func (m mockedDbConn) RunTestQuery() (err error) {
	return fmt.Errorf("Mocked run test query")
}

func (m mockedDbConn) GetConnection() (db *sqlx.DB, err error) {
	return nil, fmt.Errorf("Mocked get connection error")
}

func TestMockedHealthcheck(t *testing.T) {
	dbConn = mockedDbConn{}
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Fail to get healthcheck", "/_health", "GET", http.StatusServiceUnavailable},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	router := mux.NewRouter()
	router.HandleFunc("/_health", HealthStatus).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "HealthStatus")
		})
	}
}

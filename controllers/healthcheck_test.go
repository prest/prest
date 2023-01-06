package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/mocks"
	"github.com/prest/prest/testutils"
)

func TestHealthStatus(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Healthcheck endpoint", "/_health", "GET", http.StatusOK},
	}

	router := mux.NewRouter()
	dbConn := SDbConnection{}
	router.HandleFunc("/_health", WrappedHealthCheck(dbConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "HealthStatus")
		})
	}
}

func TestMockedHealthcheckFailedConnection(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Healthcheck endpoint failed connection", "/_health", "GET", http.StatusServiceUnavailable},
	}
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockedConn := mocks.NewMockDbConnection(mockCtrl)
	mockedConn.EXPECT().GetConnection().Return(nil, fmt.Errorf("Mocked Connection Failed"))
	router := mux.NewRouter()
	router.HandleFunc("/_health", WrappedHealthCheck(mockedConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "HealthStatus")
		})
	}
}

func TestMockedHealthcheckFailedQuery(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Healthcheck endpoint failed connection", "/_health", "GET", http.StatusServiceUnavailable},
	}
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockedConn := mocks.NewMockDbConnection(mockCtrl)
	mockedConn.EXPECT().GetConnection().Return(nil, nil)
	mockedConn.EXPECT().RunTestQuery().Return(fmt.Errorf("Failed querying"))
	router := mux.NewRouter()
	router.HandleFunc("/_health", WrappedHealthCheck(mockedConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "HealthStatus")
		})
	}
}

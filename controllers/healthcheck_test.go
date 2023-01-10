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
	router := mux.NewRouter()
	dbConn := DBConn{}
	router.HandleFunc("/_health", WrappedHealthCheck(dbConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_health", nil, "GET", http.StatusOK, "HealthStatus")
}

func TestMockedHealthcheckFailedConnection(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockedConn := mocks.NewMockDbConnection(mockCtrl)
	mockedConn.EXPECT().GetConnection().Return(nil, fmt.Errorf("Mocked Connection Failed"))
	router := mux.NewRouter()
	router.HandleFunc("/_health", WrappedHealthCheck(mockedConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_health", nil, "GET", http.StatusServiceUnavailable, "HealthStatus")
}

func TestMockedHealthcheckFailedQuery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockedConn := mocks.NewMockDbConnection(mockCtrl)
	mockedConn.EXPECT().GetConnection().Return(nil, nil)
	mockedConn.EXPECT().RunTestQuery().Return(fmt.Errorf("Failed querying"))
	router := mux.NewRouter()
	router.HandleFunc("/_health", WrappedHealthCheck(mockedConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_health", nil, "GET", http.StatusServiceUnavailable, "HealthStatus")
}

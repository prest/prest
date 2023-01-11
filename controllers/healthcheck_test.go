package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/controllers/mocks"
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

func TestMockedHealthcheckFailedQuery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockedConn := mocks.NewMockDbConnection(mockCtrl)
	mockedConn.EXPECT().ConnectionTest().Return(fmt.Errorf("Failed querying"))
	router := mux.NewRouter()
	router.HandleFunc("/_health", WrappedHealthCheck(mockedConn)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_health", nil, "GET", http.StatusServiceUnavailable, "HealthStatus")
}

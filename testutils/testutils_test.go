package testutils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestDoRequest(t *testing.T) {
	router := mux.NewRouter()
	bodyResponse := `{"key": "value"}`
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(bodyResponse))
	}).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	DoRequest(t, server.URL, nil, "GET", http.StatusOK, "")
	DoRequest(t, server.URL+"/not-found", nil, "GET", http.StatusNotFound, "")
	DoRequest(t, server.URL+"/", nil, "POST", http.StatusMethodNotAllowed, "")
	DoRequest(t, server.URL+"/", nil, "GET", http.StatusOK, "", bodyResponse)
}

func TestContainsStringInSlice(t *testing.T) {
	testCases := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{
			name:     "contains",
			slice:    []string{"a", "b", "c"},
			value:    "b",
			expected: true,
		},
		{
			name:     "not contains",
			slice:    []string{"a", "b", "c"},
			value:    "d",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := containsStringInSlice(tc.slice, tc.value)
			if actual != tc.expected {
				t.Errorf("expected %v, actual %v", tc.expected, actual)
			}
		})
	}
}

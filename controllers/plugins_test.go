package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	cmock "github.com/prest/prest/cache/mockgen"
	"github.com/prest/prest/plugins"
	"github.com/prest/prest/plugins/mockgen"
)

func Test_Plugin_ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	ml := mockgen.NewMockLoader(ctrl)

	ctrl2 := gomock.NewController(t)
	cm := cmock.NewMockCacher(ctrl2)

	ml.EXPECT().LoadFunc("testfile", "testfunc", gomock.Any()).
		Return(plugins.PluginFuncReturn{
			ReturnJson: `{"msg":"test response"}`,
			StatusCode: http.StatusOK,
		}, nil)

	cm.EXPECT().Set(gomock.Any(), gomock.Any())

	// Create a new instance of the Config struct
	config := &Config{
		pluginLoader: ml,
		cache:        cm,
	}

	// Create a new request with the desired URL path parameters
	req, err := http.NewRequest("GET", "/plugin/{file}/{func}", nil)
	require.NoError(t, err)

	// Set the URL path parameters
	vars := map[string]string{
		"file": "testfile",
		"func": "testfunc",
	}
	req = mux.SetURLVars(req, vars)

	// Create a new response recorder
	rr := httptest.NewRecorder()

	// Call the Plugin function
	config.Plugin(rr, req)

	// Check the response status code
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "test response")
}

func Test_Plugin_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	ml := mockgen.NewMockLoader(ctrl)

	ml.EXPECT().LoadFunc("testfile", "testfunc", gomock.Any()).
		Return(plugins.PluginFuncReturn{}, errors.New("plugin not found"))

	// Create a new instance of the Config struct
	config := &Config{
		pluginLoader: ml,
	}

	// Create a new request with the desired URL path parameters
	req, err := http.NewRequest("GET", "/plugin/{file}/{func}", nil)
	require.NoError(t, err)

	// Set the URL path parameters
	vars := map[string]string{
		"file": "testfile",
		"func": "testfunc",
	}
	req = mux.SetURLVars(req, vars)

	// Create a new response recorder
	rr := httptest.NewRecorder()

	// Call the Plugin function
	config.Plugin(rr, req)

	// Check the response status code
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Contains(t, rr.Body.String(), "not found")
}

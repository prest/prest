package config

import "testing"

func TestInitApp(t *testing.T) {
	initApp()
	if app == nil {
		t.Errorf("App should not be nil.")
	}
}

func TestGetApp(t *testing.T) {
	app = nil
	n := GetApp()
	if n == nil {
		t.Errorf("Should return an app.")
	}
}

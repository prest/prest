package config

import "testing"

func TestInitRouter(t *testing.T) {
	initRouter()
	if Router == nil {
		t.Errorf("Router should not be nil.")
	}
}

func TestGetRouter(t *testing.T) {
	Router = nil
	r := GetRouter()
	if r == nil {
		t.Errorf("Should return a router.")
	}
}

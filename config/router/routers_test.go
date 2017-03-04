package router

import "testing"

func TestInitRouter(t *testing.T) {
	initRouter()
	if router == nil {
		t.Errorf("Router should not be nil.")
	}
}

func TestGet(t *testing.T) {
	router = nil
	r := Get()
	if r == nil {
		t.Errorf("Should return a router.")
	}
}

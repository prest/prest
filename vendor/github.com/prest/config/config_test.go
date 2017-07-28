package config

import (
	"testing"

	"os"
)

func TestLoad(t *testing.T) {
	os.Setenv("PREST_CONF", "testdata/prest.toml")

	Load()
	if len(PrestConf.AccessConf.Tables) < 2 {
		t.Errorf("expected > 2, got: %d", len(PrestConf.AccessConf.Tables))
	}

	Load()
	if !PrestConf.AccessConf.Restrict {
		t.Error("expected true, but got false")
	}
}

func TestParse(t *testing.T) {
	os.Setenv("PREST_CONF", "testdata/prest.toml")
	viperCfg()
	cfg := &Prest{}
	err := Parse(cfg)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.HTTPPort != 6000 {
		t.Errorf("expected port: 6000, got: %d", cfg.HTTPPort)
	}

	if cfg.PGDatabase != "prest" {
		t.Errorf("expected database: prest, got: %s", cfg.PGDatabase)
	}

	os.Setenv("PREST_CONF", "../prest.toml")
	os.Setenv("PREST_HTTP_PORT", "4000")
	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)

	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}

	os.Setenv("PREST_CONF", "")
	os.Setenv("PREST_HTTP_PORT", "4000")
	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)

	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}

	os.Setenv("PREST_HTTP_PORT", "4000")
	os.Setenv("PREST_CONF", "testdata/prest.toml")
	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)

	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}
}

func TestGetDefaultPrestConf(t *testing.T) {
	prestConf := ""
	if conf := getDefaultPrestConf(prestConf); conf != "./prest.toml" {
		t.Errorf("expected ./prest.toml, but got: %q", conf)
	}

	prestConf = "../prest.toml"
	if conf := getDefaultPrestConf(prestConf); conf != "../prest.toml" {
		t.Errorf("expected ../prest.toml, but got: %q", conf)
	}
}

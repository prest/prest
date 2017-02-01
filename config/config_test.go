package config

import (
	"testing"

	"os"
)

func TestInitConf(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")

	InitConf()
	if len(PREST_CONF.AccessConf.Tables) < 2 {
		t.Errorf("expected > 2, got: %d", len(PREST_CONF.AccessConf.Tables))
	}

	InitConf()
	if !PREST_CONF.AccessConf.Restrict {
		t.Error("expected true, but got false")
	}
}

func TestParse(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
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

	os.Setenv("PREST_HTTP_PORT", "4000")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
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

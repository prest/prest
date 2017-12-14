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

	if !cfg.EnableDefaultJWT {
		t.Error("expected true but got false")
	}

	os.Setenv("PREST_CONF", "")
	os.Setenv("PREST_HTTP_PORT", "4000")
	os.Setenv("PREST_JWT_DEFAULT", "false")
	viperCfg()
	cfg = &Prest{}
	err = Parse(cfg)

	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}

	if cfg.HTTPPort != 4000 {
		t.Errorf("expected port: 4000, got: %d", cfg.HTTPPort)
	}

	if cfg.EnableDefaultJWT {
		t.Error("expected false but got true")
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
	testCases := []struct {
		name        string
		defaultFile string
		prestConf   string
		result      string
	}{
		{"empty config", "./prest.toml", "", ""},
		{"custom config", "./prest.toml", "../prest.toml", "../prest.toml"},
		{"default config", "./testdata/prest.toml", "", "./testdata/prest.toml"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultFile = tc.defaultFile
			cfg := getDefaultPrestConf(tc.prestConf)
			if cfg != tc.result {
				t.Errorf("expected %v, but got %v", tc.result, cfg)
			}
		})
	}
}

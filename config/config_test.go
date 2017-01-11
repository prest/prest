package config

import (
	"testing"

	"os"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInitConf(t *testing.T) {
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	Convey("Check tables parser", t, func() {
		InitConf()
		So(len(PREST_CONF.AccessConf.Tables), ShouldBeGreaterThanOrEqualTo, 2)
	})
	Convey("Check restrict parser", t, func() {
		InitConf()
		So(len(PREST_CONF.AccessConf.Restrict), ShouldBeTrue)
	})
}

func TestParse(t *testing.T) {
	Convey("Verify if get toml", t, func() {
		os.Setenv("PREST_CONF", "../testdata/prest.toml")
		viperCfg()
		cfg := &Prest{}
		err := Parse(cfg)
		So(err, ShouldBeNil)
		So(cfg.HTTPPort, ShouldEqual, 6000)
		So(cfg.PGDatabase, ShouldEqual, "prest")
	})
	Convey("Verify if get env", t, func() {
		os.Setenv("PREST_CONF", "../prest.toml")
		os.Setenv("PREST_HTTP_PORT", "4000")
		viperCfg()
		cfg := &Prest{}
		err := Parse(cfg)
		So(err, ShouldBeNil)
		So(cfg.HTTPPort, ShouldEqual, 4000)
	})
	Convey("Verify if env override toml", t, func() {
		os.Setenv("PREST_HTTP_PORT", "4000")
		os.Setenv("PREST_CONF", "../testdata/prest.toml")
		viperCfg()
		cfg := &Prest{}
		err := Parse(cfg)
		So(err, ShouldBeNil)
		So(cfg.HTTPPort, ShouldEqual, 4000)
	})
}

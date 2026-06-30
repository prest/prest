package controllers

import "github.com/prest/prest/v2/config"

func testHandlers() *Handlers {
	return NewHandlersFromConfig(config.PrestConf)
}

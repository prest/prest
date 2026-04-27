package main

import (
	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
)

func main() {
	config.Load()
	cmd.Execute()
}

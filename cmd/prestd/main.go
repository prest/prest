package main

import (
	"github.com/prest/prest/cmd"
	"github.com/prest/prest/config"
)

func main() {
	config.Load()
	cmd.Execute()
}

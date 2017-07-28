package main

import (
	"github.com/prest/cmd"
	"github.com/prest/config"
)

func main() {
	config.Load()
	cmd.Execute()
}

package main

import (
	"github.com/nuveo/prest/cmd"
	"github.com/nuveo/prest/config"
)

func main() {
	config.Load()
	cmd.Execute()
}

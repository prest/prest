package main

import (
	"github.com/palevi67/prest/cmd"
	"github.com/palevi67/prest/config"
)

func main() {
	config.Load()
	cmd.Execute()
}

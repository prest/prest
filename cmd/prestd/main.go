package main

import (
	"fmt"
	"plugin"

	"github.com/prest/prest/cmd"
	"github.com/prest/prest/config"
)

func main() {

	// TODO: POC load lib (.so)
	p, err := plugin.Open("./exts/lib/example.so")
	if err != nil {
		panic(err)
	}

	// string Hello is function name
	f, err := p.Lookup("Hello")
	if err != nil {
		panic(err)
	}

	// Exec (call) function name
	ret := f.(func() string)()
	// Print function return
	fmt.Println("ret plugin:", ret)

	config.Load()
	cmd.Execute()
}

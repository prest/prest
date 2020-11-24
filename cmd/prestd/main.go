package main

import (
	"fmt"
	"plugin"

	"github.com/prest/prest/cmd"
	"github.com/prest/prest/config"
)

func main() {

	// TODO: POC load lib (.so)
	p, err := plugin.Open("./exts/lib/hello.so")
	if err != nil {
		panic(err)
	}

	f, err := p.Lookup("Hello")
	if err != nil {
		panic(err)
	}
	ret := f.(func() string)()
	fmt.Println("ret plugin:", ret)

	config.Load()
	cmd.Execute()
}

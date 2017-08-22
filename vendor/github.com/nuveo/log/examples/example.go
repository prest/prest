package main

import (
	"fmt"

	"github.com/nuveo/log"
)

func main() {
	log.Debugln("Debug message")

	log.DebugMode = false
	log.Debugln("Debug message that will be hidden")

	log.Println("Info message")
	log.Warningln("Warning message")
	log.Errorln("Error message")
	log.Fatal("Fatal error message")
	fmt.Println("I will never be printed because of Fatal()")
}

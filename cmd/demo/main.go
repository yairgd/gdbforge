package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	for _, a := range os.Args[1:] {
		if a == "-version" || a == "--version" {
			fmt.Println(version)
			os.Exit(0)
		}
	}
	app, err := NewDemoApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "demo: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()
	app.Run()
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	app, err := NewDebuggerApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start debugger: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()
	app.Run()
}

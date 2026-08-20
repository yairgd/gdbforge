package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// version is set at link time by release builds / task build from a git tag:
//
//	go build -ldflags "-X main.version=v1.0.0" ./cmd/gdbforge
//
// Default "dev" means a non-release binary (:b about shows "not for release").
var version = "dev"

func main() {
	if wantsVersion(os.Args[1:]) {
		fmt.Println(version)
		os.Exit(0)
	}

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
		fmt.Fprintf(os.Stderr, "gdbforge: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()
	app.Run()
}

func wantsVersion(args []string) bool {
	for _, a := range args {
		if a == "-version" || a == "--version" {
			return true
		}
		if a == "--" {
			return false
		}
	}
	return false
}

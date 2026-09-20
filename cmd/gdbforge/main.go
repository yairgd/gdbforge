package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/yairgd/gdbforge/internal/app"
	"github.com/yairgd/gdbforge/internal/ttyhold"
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
	if ttyhold.Wants(os.Args[1:]) {
		os.Exit(ttyhold.Run(os.Args[1:]))
	}
	cfg, err := app.ParseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	dbg, err := app.NewDebuggerApp(cfg, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gdbforge: %v\n", err)
		os.Exit(1)
	}
	defer dbg.Close()
	dbg.Run()
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

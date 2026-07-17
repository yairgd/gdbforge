package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/yairgd/cgdb-go/internal/gdb"
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

	client, outputChan, err := gdb.NewGDBClient(cfg.GDBPath, cfg.Prog, cfg.ProgArgs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start gdb: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	app := NewDebuggerApp(cfg, client, outputChan)
	defer func() {
		if app.execClient != nil {
			app.execClient.Close()
		}
	}()
	app.Run()
}

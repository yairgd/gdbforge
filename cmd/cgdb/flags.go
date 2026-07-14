package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// SessionConfig holds CLI options for a debugger session.
type SessionConfig struct {
	GDBPath  string
	Prog     string
	ProgArgs []string
}

func parseFlags(args []string) (SessionConfig, error) {
	fs := flag.NewFlagSet("cgdb", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	gdbPath := fs.String("d", "gdb", "path to the gdb binary")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cgdb [-d gdb] prog [args...]\n")
		fs.SetOutput(os.Stderr)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return SessionConfig{}, err
		}
		fs.Usage()
		return SessionConfig{}, err
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		return SessionConfig{}, errors.New("missing prog")
	}

	cfg := SessionConfig{
		GDBPath: *gdbPath,
		Prog:    rest[0],
	}
	if len(rest) > 1 {
		cfg.ProgArgs = append([]string{}, rest[1:]...)
	}
	return cfg, nil
}

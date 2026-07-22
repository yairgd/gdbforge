package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// SessionConfig holds CLI options for a debugger session.
//
// Usage (cgdb-style):
//
//	gdbforge [gdbforge options] [--] [gdb options]
//
// Examples:
//
//	gdbforge ./hello
//	gdbforge -d /usr/bin/gdb ./hello a b
//	gdbforge -d /usr/bin/gdb -- -nx -x r5_debug.gdb ./zephyr.elf
type SessionConfig struct {
	GDBPath string
	// GDBArgs are arguments after the gdb binary. gdbforge always injects
	// --interpreter=mi2 before them (same role as cgdb's "[gdb options]").
	GDBArgs []string
	// Prog is a best-effort display hint (last non-flag arg); may be empty.
	Prog string
	// ProgArgs is kept for tests/legacy callers; unused when GDBArgs is set
	// via -- pass-through. Prefer GDBArgs.
	ProgArgs []string
}

func splitDashDash(args []string) (before, after []string, found bool) {
	for i, a := range args {
		if a == "--" {
			return append([]string{}, args[:i]...), append([]string{}, args[i+1:]...), true
		}
	}
	return args, nil, false
}

func inferProg(gdbArgs []string) string {
	for i := len(gdbArgs) - 1; i >= 0; i-- {
		a := gdbArgs[i]
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		// Skip values of common option pairs (-x FILE, -ex CMD, …) when the
		// previous token is a flag that takes an argument.
		if i > 0 {
			prev := gdbArgs[i-1]
			switch prev {
			case "-x", "-ex", "-iex", "-ix", "-e", "-se", "-s", "-symbols",
				"-readnow", "-r", "-d", "-cd", "-tty", "-b", "-l", "-w":
				continue
			}
			if strings.HasPrefix(prev, "-x=") || strings.HasPrefix(prev, "-ex=") {
				continue
			}
		}
		return a
	}
	return ""
}

func parseFlags(args []string) (SessionConfig, error) {
	before, after, passThrough := splitDashDash(args)

	fs := flag.NewFlagSet("gdbforge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	gdbPath := fs.String("d", "gdb", "path to the gdb binary")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gdbforge [gdbforge options] [--] [gdb options]\n\n")
		fmt.Fprintf(os.Stderr, "gdbforge Options:\n")
		fmt.Fprintf(os.Stderr, "  -d path     Debugger to use (default \"gdb\")\n")
		fmt.Fprintf(os.Stderr, "  -h, --help  Print help and exit\n")
		fmt.Fprintf(os.Stderr, "  --          End of gdbforge options; rest are passed to gdb\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  gdbforge ./hello\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -d /usr/bin/gdb ./hello a b\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -d /usr/bin/gdb -- -nx -x ./r5_debug.gdb ./zephyr.elf\n")
	}

	if err := fs.Parse(before); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return SessionConfig{}, err
		}
		fs.Usage()
		return SessionConfig{}, err
	}

	cfg := SessionConfig{GDBPath: *gdbPath}

	if passThrough {
		if len(after) == 0 {
			fs.Usage()
			return SessionConfig{}, errors.New("missing gdb options after --")
		}
		cfg.GDBArgs = after
		cfg.Prog = inferProg(after)
		return cfg, nil
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		return SessionConfig{}, errors.New("missing prog")
	}
	cfg.Prog = rest[0]
	if len(rest) > 1 {
		cfg.ProgArgs = append([]string{}, rest[1:]...)
		cfg.GDBArgs = append([]string{"--args", rest[0]}, rest[1:]...)
	} else {
		cfg.GDBArgs = []string{rest[0]}
	}
	return cfg, nil
}

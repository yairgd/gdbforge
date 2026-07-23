package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Backend kinds selected with -g (default gdb).
const (
	BackendGDB = "gdb"
	BackendDLV = "dlv"
)

// SessionConfig holds CLI options for a debugger session.
//
// Usage (cgdb-style):
//
//	gdbforge [gdbforge options] [--] [debugger options]
//
// Examples:
//
//	gdbforge ./hello
//	gdbforge -g dlv ./hello
//	gdbforge -d /usr/bin/gdb ./hello a b
//	gdbforge -d /usr/bin/gdb -- -nx -x r5_debug.gdb ./zephyr.elf
//	gdbforge -g dlv -d /usr/local/bin/dlv ./hello
type SessionConfig struct {
	// Kind is the backend: gdb or dlv (from -g).
	Kind string
	// GDBPath is the debugger binary path (from -d). Name kept for GDB-era call sites.
	GDBPath string
	// GDBArgs are arguments after the debugger binary.
	// For gdb: injected after --interpreter=mi2 (cgdb-style "[gdb options]").
	// For dlv: program and args after `dlv exec --`.
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

func normalizeKind(k string) (string, error) {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case BackendGDB, BackendDLV:
		return k, nil
	case "":
		return BackendGDB, nil
	default:
		return "", fmt.Errorf("unknown backend %q (want gdb or dlv)", k)
	}
}

func parseFlags(args []string) (SessionConfig, error) {
	before, after, passThrough := splitDashDash(args)

	fs := flag.NewFlagSet("gdbforge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	kind := fs.String("g", BackendGDB, "backend kind: gdb or dlv")
	// Empty default: filled from -g when -d is omitted.
	debuggerPath := fs.String("d", "", "path to the debugger binary (default: matches -g)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gdbforge [gdbforge options] [--] [debugger options]\n\n")
		fmt.Fprintf(os.Stderr, "gdbforge Options:\n")
		fmt.Fprintf(os.Stderr, "  -g kind     Backend: gdb or dlv (default \"gdb\")\n")
		fmt.Fprintf(os.Stderr, "  -d path     Debugger binary (default: gdb or dlv matching -g)\n")
		fmt.Fprintf(os.Stderr, "  -version    Print version and exit\n")
		fmt.Fprintf(os.Stderr, "  -h, --help  Print help and exit\n")
		fmt.Fprintf(os.Stderr, "  --          End of gdbforge options; rest passed to the debugger\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  gdbforge ./hello\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -g dlv ./hello\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -d /usr/bin/gdb ./hello a b\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -g dlv -d /usr/local/bin/dlv ./pkg\n")
		fmt.Fprintf(os.Stderr, "  gdbforge -d /usr/bin/gdb -- -nx -x ./r5_debug.gdb ./zephyr.elf\n")
	}

	if err := fs.Parse(before); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return SessionConfig{}, err
		}
		fs.Usage()
		return SessionConfig{}, err
	}

	backend, err := normalizeKind(*kind)
	if err != nil {
		fs.Usage()
		return SessionConfig{}, err
	}

	path := strings.TrimSpace(*debuggerPath)
	if path == "" {
		path = backend
	}

	cfg := SessionConfig{Kind: backend, GDBPath: path}

	if passThrough {
		if len(after) == 0 {
			fs.Usage()
			return SessionConfig{}, errors.New("missing debugger options after --")
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
		if backend == BackendDLV {
			// dlv exec -- prog args...
			cfg.GDBArgs = append([]string{rest[0]}, rest[1:]...)
		} else {
			cfg.GDBArgs = append([]string{"--args", rest[0]}, rest[1:]...)
		}
	} else {
		cfg.GDBArgs = []string{rest[0]}
	}
	return cfg, nil
}

func (c SessionConfig) IsDLV() bool {
	return c.Kind == BackendDLV
}

func (c SessionConfig) IsGDB() bool {
	return c.Kind == BackendGDB || c.Kind == ""
}

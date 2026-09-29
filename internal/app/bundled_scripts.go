package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	bundled "github.com/yairgd/gdbforge/scripts"
)

// Bundled scripts are host-side helpers that run instead of a debug session,
// not inside one. They live here rather than in cmd/gdbforge because the entry
// point may only import internal/app and internal/ttyhold.

// WantsRunScript reports a --run-script invocation and splits argv at the
// script name: everything after the name belongs to the script and is returned
// untouched, so its own --help and --version reach it rather than being read as
// gdbforge's. A missing name still returns true, so that RunBundledScript is
// the single place that reports it.
func WantsRunScript(args []string) (string, []string, bool) {
	for i, a := range args {
		if a == "--" {
			return "", nil, false
		}
		if a != "--run-script" && a != "-run-script" {
			continue
		}
		if i+1 >= len(args) {
			return "", nil, true
		}
		return args[i+1], args[i+2:], true
	}
	return "", nil, false
}

// WantsListScripts reports a --list-scripts invocation.
func WantsListScripts(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--list-scripts" || a == "-list-scripts" {
			return true
		}
	}
	return false
}

// ListBundledScripts prints the bundled script catalog.
func ListBundledScripts(out io.Writer) int {
	list := bundled.List()
	if len(list) == 0 {
		fmt.Fprintln(out, "No bundled scripts in this build.")
		return 0
	}
	width := 0
	for _, s := range list {
		if n := len(s.Name); n > width {
			width = n
		}
	}
	fmt.Fprintln(out, "Bundled scripts:")
	for _, s := range list {
		fmt.Fprintf(out, "  %-*s  %s\n", width, s.Name, s.Desc)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run one with:  gdbforge --run-script NAME [ARGS...]")
	fmt.Fprintln(out, "Each script carries its own --help for its arguments.")
	return 0
}

// RunBundledScript unpacks the named script and runs it with args, handing back
// its exit code.
//
// Bundling a script does not remove what it needs: its external tools still
// have to be installed, and zynqmp-park-el3.sh still needs the JTAG cable to
// itself, which is why this runs before the TUI and any probe server exist.
//
// The three streams are *os.File rather than io.Reader/io.Writer so that
// os/exec hands the descriptors to the child directly. Behind a generic
// interface it would insert a pipe and a copying goroutine instead, and the
// script would no longer see a terminal.
func RunBundledScript(name string, args []string, in, out, errOut *os.File) int {
	s, err := bundled.Lookup(name)
	if err != nil {
		fmt.Fprintf(errOut, "gdbforge: %v\n", err)
		return 2
	}
	path, err := bundled.Extract(s.Name)
	if err != nil {
		fmt.Fprintf(errOut, "gdbforge: cannot unpack %s: %v\n", s.Name, err)
		return 1
	}
	warnMissingTools(s, errOut)

	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errOut
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errOut, "gdbforge: cannot run %s: %v\n", path, err)
		if errors.Is(err, os.ErrPermission) {
			return 126
		}
		return 127
	}
	return 0
}

// warnMissingTools names required tools that are not on PATH without stopping
// the run: --help and --dry-run do not need them, and once the script does run
// its own diagnostics are more specific than anything that can be said here.
func warnMissingTools(s bundled.Script, errOut *os.File) {
	var missing []string
	for _, tool := range s.Requires {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	if len(missing) == 0 {
		return
	}
	fmt.Fprintf(errOut, "gdbforge: %s needs %s on PATH, not found there.\n",
		s.Name, strings.Join(missing, ", "))
	fmt.Fprintln(errOut, "gdbforge: running anyway — --help and --dry-run do not need it.")
}

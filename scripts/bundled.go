// Package bundled ships host-side helper scripts inside the gdbforge binary, so
// that a release build carries them without a source checkout.
//
// This file lives in scripts/ rather than somewhere under internal/ because
// go:embed cannot reach outside its own directory, and keeping it here is what
// lets the scripts stay at the paths the documentation already names.
//
// The embed list is explicit rather than a *.sh glob: check_imports.sh is a
// development script that must not ship, and go:embed has no exclude syntax.
package bundled

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed zynqmp-park-el3.sh rtt.sh
var FS embed.FS

// Script is one bundled script.
type Script struct {
	Name     string   // file name as embedded, e.g. "zynqmp-park-el3.sh"
	Desc     string   // one-line summary for --list-scripts
	Requires []string // external tools the script needs on PATH
}

// catalog carries the descriptions. The set of scripts itself comes from the
// embedded FS, so a new script only has to be named in the go:embed directive
// above and described here — nothing else needs to learn about it.
var catalog = []Script{
	{
		Name:     "zynqmp-park-el3.sh",
		Desc:     "Park a ZynqMP A53 core at EL3 with psu_init done, for bare-metal JTAG debug",
		Requires: []string{"xsdb"},
	},
	{
		Name:     "rtt.sh",
		Desc:     "Open the target's SEGGER RTT console as a terminal, over the debugger's JTAG cable",
		Requires: []string{"socat", "ss", "minicom"},
	},
}

// List returns every embedded script with its catalog entry merged in, ordered
// by name because fs.ReadDir sorts. A script that is embedded but not yet
// described still appears, rather than going silently missing.
func List() []Script {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return nil
	}
	out := make([]Script, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		s := Script{Name: e.Name(), Desc: "(no description)"}
		for _, c := range catalog {
			if c.Name == e.Name() {
				s = c
				break
			}
		}
		out = append(out, s)
	}
	return out
}

// Lookup resolves a user-supplied name against the bundled scripts only; the
// ".sh" suffix is optional. A name that is not a bare file name is rejected
// before any lookup, so no argument can name a path outside the bundle.
func Lookup(name string) (Script, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Script{}, fmt.Errorf("no script name given (try --list-scripts)")
	}
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		return Script{}, fmt.Errorf("%q is not a bundled script name", name)
	}
	all := List()
	for _, s := range all {
		if s.Name == name || strings.TrimSuffix(s.Name, ".sh") == name {
			return s, nil
		}
	}
	avail := make([]string, 0, len(all))
	for _, s := range all {
		avail = append(avail, s.Name)
	}
	return Script{}, fmt.Errorf("no bundled script %q (available: %s)", name, strings.Join(avail, ", "))
}

// Extract writes the script to the user cache and returns its path. It rewrites
// on every call so the copy on disk always matches this binary.
func Extract(name string) (string, error) {
	s, err := Lookup(name)
	if err != nil {
		return "", err
	}
	data, err := FS.ReadFile(s.Name)
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "gdbforge", "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, s.Name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		return "", err
	}
	// Not redundant: WriteFile applies its mode only when it creates the file,
	// and umask can clear bits when it does. Without this, an upgrade over a
	// copy that is already there keeps whatever mode that one had.
	if err := os.Chmod(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

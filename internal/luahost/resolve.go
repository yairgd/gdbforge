package luahost

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Script origins for ResolveLuaScripts (search order / logging).
const (
	OriginProject  = "project"  // ./.gdbforge/lua
	OriginHome     = "home"     // ~/.gdbforge/lua
	OriginEmbedded = "embedded" // shipped catalog (materialized cache)
)

// GdbforgeDir is the config directory name under cwd or $HOME.
const GdbforgeDir = ".gdbforge"

// ResolvedScript is one :lua command after 3-layer merge (first basename wins).
type ResolvedScript struct {
	Path   string // filesystem path to the .lua file
	Cmd    string // :lua command name (basename without .lua)
	Origin string // OriginProject, OriginHome, or OriginEmbedded
}

// WalkLuaScriptsFS recursively finds *.lua under root in fsys.
// Command name is the file basename (same rules as WalkLuaScripts).
func WalkLuaScriptsFS(fsys fs.FS, root string) ([]ScriptFile, error) {
	if root == "" {
		root = "."
	}
	var out []ScriptFile
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".lua") {
			return nil
		}
		cmd := strings.TrimSuffix(name, filepath.Ext(name))
		if cmd == "" {
			return nil
		}
		out = append(out, ScriptFile{Path: path, Cmd: cmd})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveLuaScripts walks project, home, then embedded catalog (after
// materializing embedded to the user cache). First basename wins.
// embedded may be nil to skip the shipped catalog layer.
func ResolveLuaScripts(embedded fs.FS) ([]ResolvedScript, error) {
	projectDir := filepath.Join(".", GdbforgeDir, UserLuaDir)
	homeDir := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeDir = filepath.Join(home, GdbforgeDir, UserLuaDir)
	}
	return ResolveLuaScriptsFrom(projectDir, homeDir, embedded)
}

// ResolveLuaScriptsFrom is like ResolveLuaScripts but with explicit roots
// (empty homeDir skips the home layer; nil embedded skips embedded).
func ResolveLuaScriptsFrom(projectDir, homeDir string, embedded fs.FS) ([]ResolvedScript, error) {
	seen := make(map[string]struct{})
	var out []ResolvedScript

	addLayer := func(files []ScriptFile, origin string) {
		for _, f := range files {
			if _, ok := seen[f.Cmd]; ok {
				continue
			}
			seen[f.Cmd] = struct{}{}
			out = append(out, ResolvedScript{
				Path:   f.Path,
				Cmd:    f.Cmd,
				Origin: origin,
			})
		}
	}

	if projectDir != "" {
		projectFiles, err := WalkLuaScripts(projectDir)
		if err != nil {
			return nil, fmt.Errorf("walk project lua %s: %w", projectDir, err)
		}
		addLayer(projectFiles, OriginProject)
	}

	if homeDir != "" {
		homeFiles, err := WalkLuaScripts(homeDir)
		if err != nil {
			return nil, fmt.Errorf("walk home lua %s: %w", homeDir, err)
		}
		addLayer(homeFiles, OriginHome)
	}

	if embedded != nil {
		cacheDir, err := MaterializeEmbeddedLua(embedded)
		if err != nil {
			return nil, fmt.Errorf("materialize embedded lua: %w", err)
		}
		embFiles, err := WalkLuaScripts(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("walk embedded lua %s: %w", cacheDir, err)
		}
		addLayer(embFiles, OriginEmbedded)
	}

	return out, nil
}

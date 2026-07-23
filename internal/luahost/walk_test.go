package luahost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkLuaScriptsNested(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("r5_debug/r5_debug.lua", "function main() end")
	mustWrite("games/snake/snake.lua", "function main() end")
	mustWrite("skip.txt", "nope")

	files, err := WalkLuaScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files: %+v", len(files), files)
	}
	cmds := map[string]bool{}
	for _, f := range files {
		cmds[f.Cmd] = true
		if filepath.Base(filepath.Dir(f.Path)) == "r5_debug" && f.Cmd != "r5_debug" {
			t.Fatalf("unexpected: %+v", f)
		}
	}
	if !cmds["r5_debug"] || !cmds["snake"] {
		t.Fatalf("cmds=%v", cmds)
	}
}

func TestLoadScriptFileSetsLuaDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "r5_debug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "r5_debug.lua")
	body := `
function main()
  gdbforge.print(gdbforge.lua_dir())
end
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()
	if err := rt.LoadScriptFile(path, "r5_debug"); err != nil {
		t.Fatal(err)
	}
	if err := rt.CallNamed("r5_debug"); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) == 0 || p.lines[0] != dir {
		t.Fatalf("lua_dir lines=%v want %q", p.lines, dir)
	}
}

func TestWalkLuaScriptsMissing(t *testing.T) {
	files, err := WalkLuaScripts(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("got %v", files)
	}
}

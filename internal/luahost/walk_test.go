package luahost

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	luacatalog "github.com/yairgd/gdbforge/lua"
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

func TestWalkLuaScriptsFS(t *testing.T) {
	fsys := fstest.MapFS{
		"r5_debug/r5_debug.lua":   &fstest.MapFile{Data: []byte("function main() end")},
		"games/snake/snake.lua":   &fstest.MapFile{Data: []byte("function main() end")},
		"r5_debug/r5_target.xml":  &fstest.MapFile{Data: []byte("<target/>")},
		"README.md":               &fstest.MapFile{Data: []byte("# hi")},
	}
	files, err := WalkLuaScriptsFS(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d: %+v", len(files), files)
	}
	cmds := map[string]bool{}
	for _, f := range files {
		cmds[f.Cmd] = true
	}
	if !cmds["r5_debug"] || !cmds["snake"] {
		t.Fatalf("cmds=%v", cmds)
	}
}

func TestResolveLuaScriptsFirstWins(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	home := filepath.Join(base, "home")
	mustWrite := func(root, rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(project, "r5_debug/r5_debug.lua", "function main() end -- project")
	mustWrite(home, "r5_debug/r5_debug.lua", "function main() end -- home")
	mustWrite(home, "remotegdb/remotegdb.lua", "function main() end -- home")

	emb := fstest.MapFS{
		"r5_debug/r5_debug.lua":     &fstest.MapFile{Data: []byte("function main() end -- emb")},
		"remotegdb/remotegdb.lua":   &fstest.MapFile{Data: []byte("function main() end -- emb")},
		"dlv_port/dlv_port.lua":     &fstest.MapFile{Data: []byte("function main() end -- emb")},
		"r5_debug/r5_target.xml":    &fstest.MapFile{Data: []byte("<target/>")},
	}

	// Point cache at temp so materialize does not touch the real user cache.
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))

	got, err := ResolveLuaScriptsFrom(project, home, emb)
	if err != nil {
		t.Fatal(err)
	}
	byCmd := map[string]ResolvedScript{}
	for _, s := range got {
		byCmd[s.Cmd] = s
	}
	if s, ok := byCmd["r5_debug"]; !ok || s.Origin != OriginProject {
		t.Fatalf("r5_debug: %+v want origin=%s", byCmd["r5_debug"], OriginProject)
	}
	if s, ok := byCmd["remotegdb"]; !ok || s.Origin != OriginHome {
		t.Fatalf("remotegdb: %+v want origin=%s", byCmd["remotegdb"], OriginHome)
	}
	if s, ok := byCmd["dlv_port"]; !ok || s.Origin != OriginEmbedded {
		t.Fatalf("dlv_port: %+v want origin=%s", byCmd["dlv_port"], OriginEmbedded)
	}
	// Embedded sidecar must exist next to materialized script.
	xml := filepath.Join(filepath.Dir(byCmd["dlv_port"].Path), "..", "r5_debug", "r5_target.xml")
	xml = filepath.Clean(xml)
	if _, err := os.Stat(xml); err != nil {
		// dlv_port is under cache/.../dlv_port; r5_debug sibling:
		xml = filepath.Join(filepath.Dir(filepath.Dir(byCmd["dlv_port"].Path)), "r5_debug", "r5_target.xml")
		if _, err := os.Stat(xml); err != nil {
			t.Fatalf("embedded sidecar missing: %v (tried %s)", err, xml)
		}
	}
}

func TestResolveEmbeddedCatalog(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))

	got, err := ResolveLuaScriptsFrom("", "", luacatalog.FS)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"r5_baremetal_jlink", "r5_openamp_jlink", "remotegdb", "dlv_ext_port", "snake", "tetris"}
	cmds := map[string]ResolvedScript{}
	for _, s := range got {
		cmds[s.Cmd] = s
		if s.Origin != OriginEmbedded {
			t.Fatalf("%s origin=%s", s.Cmd, s.Origin)
		}
	}
	for _, name := range want {
		if _, ok := cmds[name]; !ok {
			t.Fatalf("missing embedded cmd %q in %v", name, cmds)
		}
	}
	// Sidecar beside cortex_r5 scripts
	dir := filepath.Dir(cmds["r5_baremetal_jlink"].Path)
	if _, err := os.Stat(filepath.Join(dir, "r5_target.xml")); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeEmbeddedLua(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
	fsys := fstest.MapFS{
		"a/a.lua":   &fstest.MapFile{Data: []byte("-- a")},
		"a/side.txt": &fstest.MapFile{Data: []byte("side")},
	}
	dest, err := MaterializeEmbeddedLua(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a", "a.lua")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a", "side.txt")); err != nil {
		t.Fatal(err)
	}
}

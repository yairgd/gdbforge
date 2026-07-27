package luahost

import (
	"os"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type memPane struct {
	lines []string
	cells map[string]rune
	w, h  int
}

func (p *memPane) AppendPrint(s string) { p.lines = append(p.lines, s) }
func (p *memPane) ClearAll() {
	p.lines = nil
	p.cells = map[string]rune{}
}
func (p *memPane) ClearCells() { p.cells = map[string]rune{} }
func (p *memPane) SetCell(x, y int, ch rune, color string) {
	if p.cells == nil {
		p.cells = map[string]rune{}
	}
	p.cells[key(x, y)] = ch
}
func (p *memPane) Size() (int, int) { return p.w, p.h }

func key(x, y int) string { return strings.Join([]string{itoa(x), itoa(y)}, ",") }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func TestPrintAndRegister(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	var gotName string
	rt := New(p, func(name string, _ *Runtime) { gotName = name })
	defer rt.Close()

	src := `
gdbforge.print("hi")
gdbforge.register("hello", function(a)
  gdbforge.print("hello " .. tostring(a))
end)
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 1 || p.lines[0] != "hi" {
		t.Fatalf("print=%v", p.lines)
	}
	if gotName != "hello" {
		t.Fatalf("register hook=%q", gotName)
	}
	if err := rt.CallNamed("hello", "world"); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 2 || p.lines[1] != "hello world" {
		t.Fatalf("after call=%v", p.lines)
	}
}

func TestSetCellAndKey(t *testing.T) {
	p := &memPane{w: 8, h: 4}
	rt := New(p, nil)
	defer rt.Close()
	src := `
function on_key(k)
  pane.set_cell(1, 1, "X", "green")
  gdbforge.print(k)
end
function on_draw()
  pane.clear()
  pane.set_cell(0, 0, "@", "yellow")
end
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	rt.DispatchDraw()
	if p.cells["0,0"] != '@' {
		t.Fatalf("cell=%v", p.cells)
	}
	rt.DispatchKey("a")
	if p.cells["1,1"] != 'X' {
		t.Fatalf("after key cells=%v", p.cells)
	}
	if len(p.lines) == 0 || p.lines[len(p.lines)-1] != "a" {
		t.Fatalf("lines=%v", p.lines)
	}
}

func TestSnakeScriptLoads(t *testing.T) {
	p := &memPane{w: 40, h: 20}
	rt := New(p, nil)
	defer rt.Close()
	if err := rt.LoadString(SnakeScript, "snake"); err != nil {
		t.Fatal(err)
	}
	rt.DispatchTick(0.2)
	rt.DispatchDraw()
	rt.DispatchKey("l")
}

func TestTetrisScriptLoads(t *testing.T) {
	p := &memPane{w: 40, h: 24}
	rt := New(p, nil)
	defer rt.Close()
	if err := rt.LoadString(TetrisScript, "tetris"); err != nil {
		t.Fatal(err)
	}
	rt.DispatchTick(0.5)
	rt.DispatchDraw()
	rt.DispatchKey("h")
}

func TestHasPaneHooksAndScriptPath(t *testing.T) {
	p := &memPane{w: 8, h: 4}
	rt := New(p, nil)
	defer rt.Close()
	if rt.HasPaneHooks() {
		t.Fatal("empty runtime should not have pane hooks")
	}
	if err := rt.LoadString(`
function on_tick(dt) end
function main() end
`, "pane"); err != nil {
		t.Fatal(err)
	}
	if !rt.HasPaneHooks() {
		t.Fatal("expected on_tick hook")
	}
	rt.SetScriptPath("/tmp/game.lua")
	if rt.ScriptPath() != "/tmp/game.lua" {
		t.Fatalf("ScriptPath=%q", rt.ScriptPath())
	}
}

func TestSetPaneUnderCallNamed(t *testing.T) {
	p1 := &memPane{w: 8, h: 4}
	p2 := &memPane{w: 8, h: 4}
	rt := New(p1, nil)
	defer rt.Close()

	rt.SetOpenBuffer(func(name string) {
		if name != "g1" {
			t.Errorf("name=%q", name)
		}
		if !rt.HasPaneHooks() {
			t.Error("HasPaneHooks under CallNamed")
		}
		rt.SetPane(p2)
		p2.AppendPrint("adopted")
	})
	if err := rt.LoadString(`
function on_key(k) end
gdbforge.register("go", function()
  gdbforge.open_buffer("g1")
end)
`, "adopt"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("go") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: SetPane/HasPaneHooks under CallNamed")
	}
	if rt.Pane() != p2 {
		t.Fatal("pane not adopted")
	}
	if len(p2.lines) == 0 || p2.lines[0] != "adopted" {
		t.Fatalf("p2 lines=%v", p2.lines)
	}
}

func TestLoadScriptFileOnly(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/clone.lua"
	src := `
function on_key(k) end
function main()
  error("main should not run from LoadScriptFileOnly alone")
end
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &memPane{w: 4, h: 4}
	rt := New(p, nil)
	defer rt.Close()
	if err := rt.LoadScriptFileOnly(path); err != nil {
		t.Fatal(err)
	}
	if rt.ScriptPath() != path {
		t.Fatalf("path=%q", rt.ScriptPath())
	}
	if !rt.HasPaneHooks() {
		t.Fatal("expected on_key")
	}
	if rt.HasNamed("clone") {
		t.Fatal("LoadScriptFileOnly must not EnsureCommand")
	}
}

// Regression: app-installed Lua funcs (luadebug) call AppendPrint while CallNamed
// holds rt.mu. Re-locking AppendPrint froze :lua dlv_ext_port.
func TestAppendPrintFromHostFuncUnderCallNamed(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()

	rt.SetGdbforgeFunc("probe", func(L *lua.LState) int {
		rt.AppendPrint("from-host-func")
		return 0
	})
	if err := rt.LoadString(`
gdbforge.register("run", function()
  gdbforge.probe()
end)
`, "probe"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("run") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: AppendPrint under CallNamed")
	}
	if len(p.lines) == 0 || p.lines[len(p.lines)-1] != "from-host-func" {
		t.Fatalf("lines=%v", p.lines)
	}
}

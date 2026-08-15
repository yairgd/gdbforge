package luahost

import (
	"context"
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

func TestEvalLinePrintsReturns(t *testing.T) {
	var got []string
	rt := New(nil, nil)
	defer rt.Close()
	rt.SetPrintSink(func(line string) { got = append(got, line) })
	if err := rt.EvalLine(`return 1, "two"`); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "1\ttwo" {
		t.Fatalf("returns=%v", got)
	}
	got = got[:0]
	if err := rt.EvalLine(`gdbforge.print("hi"); return 3`); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "hi" || got[1] != "3" {
		t.Fatalf("mixed=%v", got)
	}
}

func TestPrintSinkPreferredOverPane(t *testing.T) {
	p := &memPane{w: 8, h: 4}
	rt := New(p, nil)
	defer rt.Close()
	var got []string
	rt.SetPrintSink(func(line string) { got = append(got, line) })
	if err := rt.LoadString(`gdbforge.print("hi")`, "t"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "hi" {
		t.Fatalf("sink=%v paneLines=%v", got, p.lines)
	}
	if len(p.lines) != 0 {
		t.Fatalf("pane should be unused while sink set: %v", p.lines)
	}
	rt.SetPrintSink(nil)
	if err := rt.LoadString(`gdbforge.print("pane")`, "t2"); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) == 0 || p.lines[len(p.lines)-1] != "pane" {
		t.Fatalf("pane lines=%v", p.lines)
	}
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

// Regression: ModeLua Draw/tick must not block forever while CallNamed holds
// rt.mu (worker open_buffer → callOnUI). That froze :lua snake after the pane opened.
func TestDispatchTickSkipsWhenCallNamedHoldsLock(t *testing.T) {
	p := &memPane{w: 8, h: 4}
	rt := New(p, nil)
	defer rt.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	rt.SetOpenBuffer(func(name string) {
		close(entered)
		<-release
	})
	if err := rt.LoadString(`
function on_tick(dt) end
gdbforge.register("go", function()
  gdbforge.open_buffer("pane")
end)
`, "tick-lock"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("go") }()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("open_buffer not reached")
	}

	tickDone := make(chan struct{})
	go func() {
		rt.DispatchTick(0.1)
		close(tickDone)
	}()
	select {
	case <-tickDone:
	case <-time.After(2 * time.Second):
		t.Fatal("DispatchTick blocked while CallNamed held rt.mu")
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CallNamed did not finish")
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

func TestCallHelp(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()

	if err := rt.CallHelp(); err == nil || !strings.Contains(err.Error(), "no help()") {
		t.Fatalf("want no help() error, got %v", err)
	}

	src := `
function help()
  gdbforge.print("usage here")
end
function main()
  gdbforge.print("main-ran")
end
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	if err := rt.EnsureCommand("demo"); err != nil {
		t.Fatal(err)
	}
	if err := rt.CallHelp(); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 1 || p.lines[0] != "usage here" {
		t.Fatalf("help print=%v", p.lines)
	}
	// CallHelp must not run main.
	if err := rt.CallNamed("demo"); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 2 || p.lines[1] != "main-ran" {
		t.Fatalf("after main lines=%v", p.lines)
	}
}

func TestSleepRespectsJobCancel(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	rt.SetJobContext(ctx)

	src := `
function main()
  gdbforge.sleep(30)
  gdbforge.print("finished")
end
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	if err := rt.EnsureCommand("slow"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("slow") }()
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("want cancelled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sleep did not abort on cancel")
	}
	for _, line := range p.lines {
		if line == "finished" {
			t.Fatal("main continued after cancel")
		}
	}
}

func TestSystemRespectsJobCancel(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	rt.SetJobContext(ctx)

	src := `
function main()
  gdbforge.system("sleep 30")
  gdbforge.print("finished")
end
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	if err := rt.EnsureCommand("sys"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("sys") }()
	time.Sleep(80 * time.Millisecond)
	cancel()
	rt.KillSystem()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("want cancelled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("system did not abort on cancel")
	}
	for _, line := range p.lines {
		if line == "finished" {
			t.Fatal("main continued after cancel")
		}
	}
}

func TestCallNamedStopsAfterCancelEvenWithoutHostAPI(t *testing.T) {
	p := &memPane{w: 40, h: 10}
	rt := New(p, nil)
	defer rt.Close()

	ctx, cancel := context.WithCancel(context.Background())
	rt.SetJobContext(ctx)

	src := `
function main()
  local n = 0
  while true do
    n = n + 1
  end
  gdbforge.print("finished")
end
`
	if err := rt.LoadString(src, "test"); err != nil {
		t.Fatal(err)
	}
	if err := rt.EnsureCommand("spin"); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- rt.CallNamed("spin") }()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want cancel error from SetContext")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Lua loop did not abort on cancel")
	}
	for _, line := range p.lines {
		if line == "finished" {
			t.Fatal("main continued after cancel")
		}
	}
}

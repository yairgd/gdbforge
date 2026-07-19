package widgets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

func TestCodeWidgetShowLocationMarksPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "int main(void) {\n  return 0;\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	w := NewCodeWidget()
	if err := w.ShowLocation(path, 2); err != nil {
		t.Fatal(err)
	}
	lines := w.LinesForTest()
	if len(lines) < 2 {
		t.Fatalf("lines=%v", lines)
	}
	plain1 := termui.StripANSI(lines[1])
	if !strings.Contains(plain1, "━━▶") {
		t.Fatalf("want ━━▶ on line 2, got %q", plain1)
	}
	if !strings.Contains(plain1, "│") {
		t.Fatalf("want box-drawing │ gutter, got %q", plain1)
	}
	plain0 := termui.StripANSI(lines[0])
	if !strings.Contains(plain0, "1") {
		t.Fatalf("want line 1 gutter, got %q", plain0)
	}
}

func TestCodeWidgetBreakpointLineNumberANSI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "int main(void) {\n  return 0;\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	w.SetBreakpointLines([]int{2})
	lines := w.LinesForTest()
	if len(lines) < 2 {
		t.Fatalf("lines=%d", len(lines))
	}
	if !strings.Contains(lines[1], "48;5;196") {
		t.Fatalf("want red bg on breakpoint line number, got %q", lines[1])
	}
	if strings.Contains(lines[0], "48;5;196") {
		t.Fatalf("line 1 should not have red bp bg: %q", lines[0])
	}
}

func TestCodeWidgetDisabledBreakpointYellow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "int main(void) {\n  return 0;\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	w.SetBreakInfos([]mcp.BreakInfo{
		{Number: 0, Enabled: false, File: path, Line: 2},
	})
	lines := w.LinesForTest()
	if len(lines) < 2 {
		t.Fatalf("lines=%d", len(lines))
	}
	if !strings.Contains(lines[1], "48;5;226") {
		t.Fatalf("want yellow bg on disabled bp line number, got %q", lines[1])
	}
	if strings.Contains(lines[1], "48;5;196") {
		t.Fatalf("disabled should not use red: %q", lines[1])
	}
}

func TestCodeWidgetMoveSel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	w.SetFocused(true)
	if err := w.ShowLocation(path, 2); err != nil {
		t.Fatal(err)
	}
	if w.SelLine() != 2 {
		t.Fatalf("sel=%d", w.SelLine())
	}
	w.moveSel(1)
	if w.SelLine() != 3 {
		t.Fatalf("sel after down=%d", w.SelLine())
	}
	w.moveSel(-10)
	if w.SelLine() != 1 {
		t.Fatalf("sel after clamp=%d", w.SelLine())
	}
}

func TestCodeWidgetSyncSelFromViewportClick(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	w.SetFocused(true)
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	w.viewport.CursorLine = 2 // 0-based → source line 3
	w.syncSelFromViewport()
	if w.SelLine() != 3 {
		t.Fatalf("mouse sync sel=%d want 3", w.SelLine())
	}
}

func TestCodeWidgetSpaceTogglesBreak(t *testing.T) {
	w := NewCodeWidget()
	sent := make(chan string, 4)
	w.sess = &fakeSess{sent: sent}
	w.path = "/home/yair/cgdb-go/hello.c"
	w.rawLines = []string{"int main() {", "  return 0;", "}"}
	w.selLine = 2

	ev := tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("space should be handled")
	}
	cmd := <-sent
	if cmd != "break hello.c:2" {
		t.Fatalf("insert cmd=%q", cmd)
	}
	if !w.hasBreakpoint(2) {
		t.Fatal("expected optimistic red mark")
	}

	if !w.HandleFocusKey(ev) {
		t.Fatal("space toggle off should be handled")
	}
	cmd = <-sent
	if cmd != "clear hello.c:2" {
		t.Fatalf("clear cmd=%q", cmd)
	}
	if w.hasBreakpoint(2) {
		t.Fatal("expected red mark cleared")
	}
}

func TestCodeWidgetSetBreakInfosNilKeepsMarks(t *testing.T) {
	w := NewCodeWidget()
	w.path = "/tmp/a.c"
	w.rawLines = []string{"a", "b"}
	w.addLocalBreak(2)
	w.SetBreakInfos(nil) // failed refresh
	if !w.hasBreakpoint(2) {
		t.Fatal("nil SetBreakInfos must keep red mark")
	}
	w.SetBreakInfos([]mcp.BreakInfo{}) // real empty
	if w.hasBreakpoint(2) {
		t.Fatal("empty SetBreakInfos must clear red mark")
	}
}

func TestCodeWidgetBreakWhileRunningInterruptsAndContinues(t *testing.T) {
	w := NewCodeWidget()
	sent := make(chan string, 8)
	w.sess = &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)
	w.state = st
	w.path = "/home/yair/cgdb-go/hello.c"
	w.rawLines = []string{"int main() {", "  return 0;", "}"}
	w.selLine = 2

	ev := tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("space should be handled")
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "break hello.c:2" {
		t.Fatalf("break=%q", got)
	}
	if got := <-sent; got != "continue" {
		t.Fatalf("continue=%q", got)
	}
}

func TestCodeWidgetClearWhileRunningDoesNotContinueByDefault(t *testing.T) {
	w := NewCodeWidget()
	sent := make(chan string, 8)
	w.sess = &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)
	w.state = st
	w.path = "/home/yair/cgdb-go/hello.c"
	w.rawLines = []string{"int main() {", "  return 0;", "}"}
	w.selLine = 2
	w.addLocalBreak(2)

	ev := tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("space should be handled")
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "clear hello.c:2" {
		t.Fatalf("clear=%q", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("unexpected send after clear: %q", got)
	default:
	}
}

func TestCodeWidgetClearWhileRunningContinuesWhenEnabled(t *testing.T) {
	w := NewCodeWidget()
	sent := make(chan string, 8)
	w.sess = &fakeSess{sent: sent}
	st := platform.NewAppState()
	st.SetInferiorRunning(true)
	st.SetContinueAfterClear(true)
	w.state = st
	w.path = "/home/yair/cgdb-go/hello.c"
	w.rawLines = []string{"int main() {", "  return 0;", "}"}
	w.selLine = 2
	w.addLocalBreak(2)

	ev := tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("space should be handled")
	}
	if got := <-sent; got != "\x03" {
		t.Fatalf("interrupt=%q", got)
	}
	if got := <-sent; got != "clear hello.c:2" {
		t.Fatalf("clear=%q", got)
	}
	if got := <-sent; got != "continue" {
		t.Fatalf("continue=%q", got)
	}
}

type fakeSess struct {
	sent chan string
}

func (f *fakeSess) Send(cmd string) error { f.sent <- cmd; return nil }
func (f *fakeSess) SendRaw(string) error  { return nil }
func (f *fakeSess) Close()                {}
func (f *fakeSess) Subscribe() (<-chan core.PtyOutputMsg, func()) {
	ch := make(chan core.PtyOutputMsg)
	return ch, func() {}
}
func (f *fakeSess) WithWrite(_ context.Context, fn func(w core.PTYWriter) error) error {
	return fn(fakePW{f})
}

type fakePW struct{ f *fakeSess }

func (p fakePW) Send(cmd string) error { p.f.sent <- cmd; return nil }
func (p fakePW) SendRaw(raw string) error {
	p.f.sent <- raw
	return nil
}

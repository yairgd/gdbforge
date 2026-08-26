package widgets

import (
	"context"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
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
	if !strings.Contains(lines[1], "━━▶") {
		t.Fatalf("want ━━▶ on line 2, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "│") {
		t.Fatalf("want box-drawing │ gutter, got %q", lines[1])
	}
	if !strings.Contains(lines[0], "1") {
		t.Fatalf("want line 1 gutter, got %q", lines[0])
	}
	for i, ln := range lines {
		if strings.Contains(ln, "\x1b") {
			t.Fatalf("buffer line %d has ANSI escape: %q", i, ln)
		}
	}
}

func TestCodeWidgetShowSelectionKeepsPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	src := "a\nb\nc\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	if err := w.ShowSelection(path, 3); err != nil {
		t.Fatal(err)
	}
	if w.PCLine() != 1 {
		t.Fatalf("PC moved to %d, want 1", w.PCLine())
	}
	if w.SelLine() != 3 {
		t.Fatalf("sel=%d want 3", w.SelLine())
	}
	if !strings.Contains(w.LinesForTest()[0], "━━▶") {
		t.Fatalf("━━▶ should remain on line 1: %q", w.LinesForTest()[0])
	}
}

func TestCodeWidgetBreakpointLineNumberStyle(t *testing.T) {
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
	// Line-number cells are visible cols 4..7.
	st := w.cellStyle(1, 5, tcell.StyleDefault)
	_, bg, _ := st.Decompose()
	if bg != tcell.ColorRed {
		t.Fatalf("want red bg on breakpoint line number, got %v", bg)
	}
	st0 := w.cellStyle(0, 5, tcell.StyleDefault)
	_, bg0, _ := st0.Decompose()
	if bg0 == tcell.ColorRed {
		t.Fatalf("line 1 should not have red bp bg")
	}
	if strings.Contains(w.LinesForTest()[1], "\x1b") {
		t.Fatalf("buffer must stay plain: %q", w.LinesForTest()[1])
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
	w.SetBreakInfos([]models.BreakInfo{
		{Number: 0, Enabled: false, File: path, Line: 2},
	})
	st := w.cellStyle(1, 5, tcell.StyleDefault)
	_, bg, _ := st.Decompose()
	if bg != tcell.ColorYellow {
		t.Fatalf("want yellow bg on disabled bp line number, got %v", bg)
	}
	if bg == tcell.ColorRed {
		t.Fatalf("disabled should not use red")
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

func TestCodeWidgetSpaceFiresBreakToggle(t *testing.T) {
	ctx := testWidgetCtx()
	var gotPath string
	var gotLine int
	platform.Subscribe(ctx.Bus, func(msg events.CodeBreakToggleMsg) {
		gotPath, gotLine = msg.Path, msg.Line
	})

	w := NewCodeWidget()
	w.Ctx = ctx
	w.path = "/home/yair/gdbforge/hello.c"
	w.rawLines = []string{"int main() {", "  return 0;", "}"}
	w.selLine = 2

	ev := tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
	if !w.HandleFocusKey(ev) {
		t.Fatal("space should be handled")
	}
	if gotPath != w.path || gotLine != 2 {
		t.Fatalf("toggle path=%q line=%d", gotPath, gotLine)
	}
}

func TestCodeWidgetSetBreakInfosNilKeepsMarks(t *testing.T) {
	w := NewCodeWidget()
	w.path = "/tmp/a.c"
	w.rawLines = []string{"a", "b"}
	w.SetBreakInfos([]models.BreakInfo{{File: "/tmp/a.c", Line: 2, Enabled: true}})
	w.SetBreakInfos(nil) // failed refresh
	if !w.HasEnabledBreak(2) {
		t.Fatal("nil SetBreakInfos must keep red mark")
	}
	w.SetBreakInfos([]models.BreakInfo{}) // real empty
	if w.HasEnabledBreak(2) {
		t.Fatal("empty SetBreakInfos must clear red mark")
	}
}

func TestCodeWidgetShowUnavailable(t *testing.T) {
	w := NewCodeWidget()
	w.ShowUnavailable("/usr/lib64/libQt5Core.so.5", "QEventLoop::exec  line 602")
	if !w.Unavailable() {
		t.Fatal("want Unavailable")
	}
	if w.PaneName != "libQt5Core.so.5" {
		t.Fatalf("PaneName=%q", w.PaneName)
	}
	if got := w.statusLabel(); got != "/usr/lib64/libQt5Core.so.5" {
		t.Fatalf("statusLabel=%q", got)
	}
	if got := w.LinesForTest(); len(got) != 0 {
		t.Fatalf("want empty buffer lines, got %v", got)
	}

	g := termui.NewGrid(40, 10)
	full := termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 40, 10))
	w.Draw(full)

	rowText := func(y int) string {
		var b strings.Builder
		for x := 0; x < 40; x++ {
			ch := g.Cells[x][y].Rune
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
		return strings.TrimSpace(b.String())
	}
	// nLines=3 → startY=(10-3)/2=3
	title := rowText(3)
	path := rowText(4)
	extra := rowText(5)
	if title != "not available" {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(path, "libQt5Core.so.5") {
		t.Fatalf("path=%q", path)
	}
	if !strings.Contains(extra, "QEventLoop::exec") || !strings.Contains(extra, "602") {
		t.Fatalf("extra=%q", extra)
	}
}

func TestCodeWidgetStatusLineFullPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.c")
	if err := os.WriteFile(path, []byte("int main(void) { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := NewCodeWidget()
	if err := w.ShowLocation(path, 1); err != nil {
		t.Fatal(err)
	}
	if got := w.statusLabel(); got != path {
		t.Fatalf("statusLabel=%q want %q", got, path)
	}
	// PaneName stays basename for :b buffer switching.
	if w.PaneName != "hello.c" {
		t.Fatalf("PaneName=%q", w.PaneName)
	}

	g := termui.NewGrid(80, 5)
	c := termui.NewCanvas(g).WithRect(termui.NewRect(0, 0, 80, 4))
	w.SetFocused(true)
	w.DrawStatusLine(c, false)
	var b strings.Builder
	for x := 0; x < 80; x++ {
		ch := g.Cells[x][4].Rune
		if ch == 0 {
			ch = ' '
		}
		b.WriteRune(ch)
	}
	got := strings.TrimSpace(b.String())
	if !strings.Contains(got, path) {
		t.Fatalf("status bar=%q want full path %q", got, path)
	}
}

func TestCodeWidgetShowLocationSharedLib(t *testing.T) {
	w := NewCodeWidget()
	if err := w.ShowLocation("/usr/lib64/libQt5Core.so.5", 602); err != nil {
		t.Fatal(err)
	}
	if !w.Unavailable() {
		t.Fatal("want Unavailable for .so path")
	}
}

func TestCodeWidgetShowLocationMissingFile(t *testing.T) {
	w := NewCodeWidget()
	missing := filepath.Join(t.TempDir(), "no-such.c")
	if err := w.ShowLocation(missing, 10); err != nil {
		t.Fatal(err)
	}
	if !w.Unavailable() {
		t.Fatal("want Unavailable for missing file")
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

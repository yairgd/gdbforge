package widgets

import (
	"fmt"
	"path/filepath"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// CallStackWidget shows GDB stack frames (read-only list; j/k selection).
type CallStackWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	items    []mcp.StackFrame
	selected int
}

func NewCallStackWidget() *CallStackWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)

	w := &CallStackWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Call Stack"},
		viewport:   vp,
		buf:        buf,
	}
	vp.RowStyle = w.rowStyle
	w.initKeyBindings()
	w.rebuild()
	return w
}

func (w *CallStackWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
}

func (w *CallStackWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(tcell.ColorGray)
	}
	if lineIdx == w.selected && w.Focused() {
		return st.Bold(true).Background(tcell.ColorDarkBlue)
	}
	_ = line
	return st
}

func (w *CallStackWidget) move(delta int) {
	n := len(w.items)
	if n == 0 {
		return
	}
	w.selected = (w.selected + delta%n + n) % n
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.Left = 0
	w.viewport.EnsureCursorVisible()
}

// SetItems replaces the frame list and rebuilds the viewport.
func (w *CallStackWidget) SetItems(items []mcp.StackFrame) {
	w.items = append([]mcp.StackFrame(nil), items...)
	w.selected = 0
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
}

func (w *CallStackWidget) rebuild() {
	w.buf.Clear()
	w.viewport.Left = 0
	if len(w.items) == 0 {
		w.buf.AppendLine("no frames")
		w.viewport.CursorLine = 0
		return
	}
	for _, it := range w.items {
		fn := it.Func
		if fn == "" {
			fn = "?"
		}
		loc := "-"
		if it.File != "" && it.Line > 0 {
			loc = fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
		}
		w.buf.AppendLine(fmt.Sprintf("%d  %s  %s", it.Level, fn, loc))
	}
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureCursorVisible()
}

func (w *CallStackWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *CallStackWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.viewport.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *CallStackWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
}

func (w *CallStackWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *CallStackWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *CallStackWidget) Items() []mcp.StackFrame {
	return append([]mcp.StackFrame(nil), w.items...)
}

func (w *CallStackWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

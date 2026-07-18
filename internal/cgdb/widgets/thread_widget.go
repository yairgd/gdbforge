package widgets

import (
	"fmt"
	"path/filepath"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// ThreadWidget shows GDB threads (read-only list; j/k selection).
type ThreadWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	items    []mcp.ThreadInfo
	selected int
}

func NewThreadWidget() *ThreadWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)

	w := &ThreadWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Threads"},
		viewport:   vp,
		buf:        buf,
	}
	vp.RowStyle = w.rowStyle
	w.initKeyBindings()
	w.rebuild()
	return w
}

func (w *ThreadWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
}

func (w *ThreadWidget) rowStyle(lineIdx int, line string) tcell.Style {
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

func (w *ThreadWidget) move(delta int) {
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

// SetItems replaces the thread list and rebuilds the viewport.
func (w *ThreadWidget) SetItems(items []mcp.ThreadInfo) {
	w.items = append([]mcp.ThreadInfo(nil), items...)
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	// Prefer current thread when present.
	for i, it := range w.items {
		if it.Current {
			w.selected = i
			break
		}
	}
	w.rebuild()
}

func (w *ThreadWidget) rebuild() {
	w.buf.Clear()
	w.viewport.Left = 0
	if len(w.items) == 0 {
		w.buf.AppendLine("no threads")
		w.viewport.CursorLine = 0
		return
	}
	for _, it := range w.items {
		loc := "-"
		if it.File != "" && it.Line > 0 {
			loc = fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
		}
		state := it.State
		if state == "" {
			state = "-"
		}
		w.buf.AppendLine(fmt.Sprintf("%s  %s  %s", it.ID, state, loc))
	}
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureCursorVisible()
}

func (w *ThreadWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *ThreadWidget) HandleEvent(ev tcell.Event) {
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

func (w *ThreadWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
}

func (w *ThreadWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *ThreadWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *ThreadWidget) Items() []mcp.ThreadInfo {
	return append([]mcp.ThreadInfo(nil), w.items...)
}

func (w *ThreadWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

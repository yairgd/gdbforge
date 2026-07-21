package widgets

import (
	"fmt"
	"path/filepath"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ThreadWidget shows GDB threads.
//
//	j/k or Up/Down — move selection and OnActivate (like Enter)
//	wheel — move highlight only (avoids flooding :b with paths)
//	Enter / click — OnActivate
type ThreadWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
	state    *platform.AppState

	items    []mcp.ThreadInfo
	selected int

	// OnActivate is called on Enter, click, or keyboard j/k / arrows.
	OnActivate func(mcp.ThreadInfo)
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

// SetAppState wires mark / mark-dim colors for the selection row.
func (w *ThreadWidget) SetAppState(st *platform.AppState) {
	w.state = st
}

func (w *ThreadWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected() }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected() }, "<Enter>", "<C-m>")
}

func (w *ThreadWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return tcell.ColorBlue
}

func (w *ThreadWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return tcell.ColorGray
}

func (w *ThreadWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(tcell.ColorGray)
	}
	if lineIdx == w.selected {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
		}
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
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
	w.viewport.EnsureLineVisible()
}

// syncSelectedFromViewport moves the bold blue selection to the mouse-clicked row.
func (w *ThreadWidget) syncSelectedFromViewport() {
	n := len(w.items)
	if n == 0 {
		return
	}
	line := w.viewport.CursorLine
	if line < 0 {
		line = 0
	}
	if line >= n {
		line = n - 1
	}
	w.selected = line
	w.viewport.CursorLine = line
}

func (w *ThreadWidget) activateSelected() {
	if w.OnActivate == nil || len(w.items) == 0 {
		return
	}
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	w.OnActivate(w.items[w.selected])
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
	w.viewport.EnsureLineVisible()
}

func (w *ThreadWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *ThreadWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		btns := e.Buttons()
		// Wheel only moves the selection highlight (not activate).
		if btns&tcell.WheelUp != 0 {
			w.move(-1)
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.move(1)
			return
		}
		w.viewport.HandleEvent(e)
		if btns&tcell.ButtonPrimary != 0 {
			w.syncSelectedFromViewport()
			w.activateSelected()
		}
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

func (w *ThreadWidget) Selected() int { return w.selected }

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

// ViewportLeftForTest exposes the horizontal scroll offset for unit tests.
func (w *ThreadWidget) ViewportLeftForTest() int {
	return w.viewport.Left
}


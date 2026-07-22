package widgets

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// BreakpointWidget is a view of the shared breakpoint model.
// The app owns Merge/Toggle/Delete and GDB sends; this widget only paints
// SetItems and fires OnActivate / OnToggle / OnDelete intents.
//
//	j/k or Up/Down — move selection and OnActivate (like Enter)
//	wheel — same (Code jumps to the breakpoint)
//	Enter / click — OnActivate
//	e — OnToggle(selected)
//	d — OnDelete(selected)
type BreakpointWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	state *platform.AppState

	items    []mcp.BreakInfo
	selected int

	// OnActivate is called when the user selects a row.
	OnActivate func(mcp.BreakInfo)
	// OnToggle is e — enable/disable at selected index.
	OnToggle func(index int)
	// OnDelete is d — remove selected index.
	OnDelete func(index int)
}

func NewBreakpointWidget() *BreakpointWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)

	w := &BreakpointWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Breakpoints"},
		viewport:   vp,
		buf:        buf,
	}
	vp.RowStyle = w.rowStyle
	w.initKeyBindings()
	w.rebuild()
	return w
}

// SetAppState wires mark / break colors for painting.
func (w *BreakpointWidget) SetAppState(st *platform.AppState) {
	w.state = st
}

// SetItems replaces the painted list (from the shared model).
func (w *BreakpointWidget) SetItems(items []mcp.BreakInfo) {
	w.items = append([]mcp.BreakInfo(nil), items...)
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
}

func (w *BreakpointWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected() }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected() }, "<Enter>", "<C-m>")
	w.BindKeyFunc("toggle", func(args ...any) {
		if w.OnToggle != nil && len(w.items) > 0 {
			w.OnToggle(w.selected)
		}
	}, "e")
	w.BindKeyFunc("delete", func(args ...any) {
		if w.OnDelete != nil && len(w.items) > 0 {
			w.OnDelete(w.selected)
		}
	}, "d")
}

func (w *BreakpointWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(w.mutedColor())
	}
	if lineIdx == w.selected {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
		}
		// Selected row at ━━▶ / stop PC stays green (not blue/magenta mark).
		if lineIdx >= 0 && lineIdx < len(w.items) && w.atProgramPoint(w.items[lineIdx]) {
			bg = w.stackBreakColor()
		}
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	if lineIdx < 0 || lineIdx >= len(w.items) {
		return st
	}
	it := w.items[lineIdx]
	// Same stack-break color as Call Stack / Threads when ━━▶ is on this BP.
	if w.atProgramPoint(it) {
		bg := w.stackBreakColor()
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	bg := w.breakColor()
	if !it.Enabled {
		bg = w.breakDisabledColor()
	}
	_ = line
	return st.Background(bg).Foreground(platform.ContrastColor(bg)).Bold(true)
}

func (w *BreakpointWidget) atProgramPoint(it mcp.BreakInfo) bool {
	if w.state == nil {
		return false
	}
	return sameSourceLoc(it.File, it.Line, w.state.StopFile(), w.state.StopLine())
}

func (w *BreakpointWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (w *BreakpointWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (w *BreakpointWidget) breakColor() tcell.Color {
	if w.state != nil {
		return w.state.BreakColor()
	}
	return platform.DefaultBreakColor
}

func (w *BreakpointWidget) breakDisabledColor() tcell.Color {
	if w.state != nil {
		return w.state.BreakDisabledColor()
	}
	return platform.DefaultBreakDisabledColor
}

func (w *BreakpointWidget) pcColor() tcell.Color {
	if w.state != nil {
		return w.state.PCColor()
	}
	return platform.DefaultPCColor
}

func (w *BreakpointWidget) stackBreakColor() tcell.Color {
	if w.state != nil {
		return w.state.StackBreakColor()
	}
	return platform.DefaultStackBreakColor
}

func (w *BreakpointWidget) mutedColor() tcell.Color {
	if w.state != nil {
		return w.state.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (w *BreakpointWidget) move(delta int) {
	n := len(w.items)
	if n == 0 {
		return
	}
	w.selected = (w.selected + delta%n + n) % n
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureLineVisible()
}

func (w *BreakpointWidget) syncSelectedFromViewport() {
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

func (w *BreakpointWidget) activateSelected() {
	if w.OnActivate == nil || len(w.items) == 0 {
		return
	}
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	w.OnActivate(w.items[w.selected])
}

func (w *BreakpointWidget) rebuild() {
	w.buf.Clear()
	w.viewport.Left = 0
	if len(w.items) == 0 {
		w.buf.AppendLine("no breakpoints")
		w.viewport.CursorLine = 0
		return
	}
	for _, it := range w.items {
		en := "n"
		if it.Enabled {
			en = "y"
		}
		num := "  -"
		if it.Number > 0 {
			num = fmt.Sprintf("%3d", it.Number)
		}
		file := it.File
		if file == "" {
			file = "?"
		}
		w.buf.AppendLine(fmt.Sprintf("%s  %s  %s:%d", num, en, file, it.Line))
	}
	w.viewport.CursorLine = w.selected
	w.viewport.CursorCol = 0
	w.viewport.EnsureCursorVisible()
}

func (w *BreakpointWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *BreakpointWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		btns := e.Buttons()
		// Wheel moves selection and activates so Code jumps to the breakpoint.
		if btns&tcell.WheelUp != 0 {
			w.move(-1)
			w.activateSelected()
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.move(1)
			w.activateSelected()
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

func (w *BreakpointWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
}

func (w *BreakpointWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *BreakpointWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *BreakpointWidget) Selected() int { return w.selected }

// SelectIndex highlights a row (e.g. after model ToggleEnableAtFileLine).
func (w *BreakpointWidget) SelectIndex(i int) {
	if len(w.items) == 0 {
		w.selected = 0
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(w.items) {
		i = len(w.items) - 1
	}
	w.selected = i
	w.rebuild()
}

func (w *BreakpointWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

package widgets

import (
	"fmt"
	"path/filepath"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// CallStackWidget shows GDB stack frames.
//
//	j/k or Up/Down — move selection and OnActivate (like Enter)
//	wheel — same (Code follows the selected frame)
//	Enter / click — OnActivate
type CallStackWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
	state    *platform.AppState

	items    []mcp.StackFrame
	selected int

	// mouseDown tracks primary-button press so we activate on release, not on
	// every drag sample (which flooded GDB with `frame N` console noise).
	mouseDown     bool
	pressSelected int
	lastActLevel  int
	lastActTime   time.Time

	// HasBreakAt reports a breakpoint at file:line (wired from BreakpointList).
	HasBreakAt func(file string, line int) bool

	// OnActivate is called on Enter, click, or keyboard j/k / arrows.
	OnActivate func(mcp.StackFrame)
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

// SetAppState wires mark / mark-dim colors for the selection row.
func (w *CallStackWidget) SetAppState(st *platform.AppState) {
	w.state = st
}

func (w *CallStackWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected() }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected() }, "<Enter>", "<C-m>")
}

func (w *CallStackWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (w *CallStackWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (w *CallStackWidget) pcColor() tcell.Color {
	if w.state != nil {
		return w.state.PCColor()
	}
	return platform.DefaultPCColor
}

func (w *CallStackWidget) stackBreakColor() tcell.Color {
	if w.state != nil {
		return w.state.StackBreakColor()
	}
	return platform.DefaultStackBreakColor
}

func (w *CallStackWidget) mutedColor() tcell.Color {
	if w.state != nil {
		return w.state.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (w *CallStackWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(w.mutedColor())
	}
	if lineIdx == w.selected {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
		}
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	// Green only for frame 0 when ━━▶ points at that frame.
	if w.isFrameZero(lineIdx) && w.atProgramPoint(lineIdx) {
		bg := w.stackBreakColor()
		_ = line
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	_ = line
	return st
}

func (w *CallStackWidget) atProgramPoint(lineIdx int) bool {
	if w.state == nil || lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	it := w.items[lineIdx]
	// Use stop PC (frame 0), not browsed CurrentLocation — mouse frame
	// clicks must not clear the green mark on #0.
	return sameSourceLoc(it.File, it.Line, w.state.StopFile(), w.state.StopLine())
}

func (w *CallStackWidget) isFrameZero(lineIdx int) bool {
	if lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	return w.items[lineIdx].Level == 0
}

func (w *CallStackWidget) atBreakOnProgramPoint(lineIdx int) bool {
	if !w.atProgramPoint(lineIdx) {
		return false
	}
	if w.HasBreakAt == nil {
		return false
	}
	it := w.items[lineIdx]
	return w.HasBreakAt(it.File, it.Line)
}

func (w *CallStackWidget) move(delta int) {
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
func (w *CallStackWidget) syncSelectedFromViewport() {
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

func (w *CallStackWidget) activateSelected() {
	if w.OnActivate == nil || len(w.items) == 0 {
		return
	}
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	fr := w.items[w.selected]
	// Collapse duplicate activates from noisy mouse press/release sequences.
	now := time.Now()
	if fr.Level == w.lastActLevel && now.Sub(w.lastActTime) < 300*time.Millisecond {
		return
	}
	w.lastActLevel = fr.Level
	w.lastActTime = now
	w.OnActivate(fr)
}

// SetItems replaces the frame list and rebuilds the viewport.
// Preserves the selected GDB frame level when still present.
func (w *CallStackWidget) SetItems(items []mcp.StackFrame) {
	prevLevel := -1
	if w.selected >= 0 && w.selected < len(w.items) {
		prevLevel = w.items[w.selected].Level
	}
	w.items = append([]mcp.StackFrame(nil), items...)
	w.selected = 0
	if prevLevel >= 0 {
		for i, it := range w.items {
			if it.Level == prevLevel {
				w.selected = i
				break
			}
		}
	}
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
}

// SelectLevel highlights the frame with the given GDB level (no OnActivate).
func (w *CallStackWidget) SelectLevel(level int) {
	for i, it := range w.items {
		if it.Level == level {
			w.selected = i
			w.viewport.CursorLine = i
			w.viewport.CursorCol = 0
			w.viewport.EnsureLineVisible()
			return
		}
	}
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
		btns := e.Buttons()
		// Wheel moves selection and activates so Code follows the frame.
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
			if !w.mouseDown {
				w.mouseDown = true
				w.pressSelected = w.selected
			}
			return
		}
		// Activate on release: skip when the gesture was a text-selection drag
		// on the same row (copy). Still activate if the row changed.
		if w.mouseDown {
			w.mouseDown = false
			w.syncSelectedFromViewport()
			if !w.viewport.HasSelection() || w.selected != w.pressSelected {
				w.activateSelected()
			}
		}
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
	if !focused {
		w.mouseDown = false
	}
}

func (w *CallStackWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *CallStackWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *CallStackWidget) Selected() int { return w.selected }

// SelectedFrame returns the highlighted stack frame, or false if none.
func (w *CallStackWidget) SelectedFrame() (mcp.StackFrame, bool) {
	if w.selected < 0 || w.selected >= len(w.items) {
		return mcp.StackFrame{}, false
	}
	return w.items[w.selected], true
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

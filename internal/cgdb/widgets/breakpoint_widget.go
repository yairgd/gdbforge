package widgets

import (
	"fmt"
	"path/filepath"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/mcp"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
)

// BreakpointWidget owns the breakpoint list shown in the UI.
//
//	j/k or Up/Down — bold selection
//	e — toggle: disabled → insert into GDB; enabled → delete from GDB (row stays)
//	d — remove from list and from GDB
//
// Disabled rows stay in this list but are not present in GDB; CodeWidget red
// marks follow EnabledBreakInfos() only.
type BreakpointWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer

	sess  core.Session
	state *platform.AppState

	items    []mcp.BreakInfo
	selected int

	// OnChange is invoked after the internal list changes (e/d or GDB merge).
	OnChange func()
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

// SetPTY wires the shared GDB session and AppState for exclusive writes.
func (w *BreakpointWidget) SetPTY(sess core.Session, state *platform.AppState) {
	w.sess = sess
	w.state = state
}

func (w *BreakpointWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
	w.BindKeyFunc("toggle", func(args ...any) { w.toggleSelected() }, "e")
	w.BindKeyFunc("delete", func(args ...any) { w.deleteSelected() }, "d")
}

func (w *BreakpointWidget) rowStyle(lineIdx int, line string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(tcell.ColorGray)
	}
	if lineIdx == w.selected && w.Focused() {
		return st.Bold(true).Background(tcell.ColorDarkBlue)
	}
	if lineIdx >= 0 && lineIdx < len(w.items) && !w.items[lineIdx].Enabled {
		return st.Foreground(tcell.ColorGray)
	}
	_ = line
	return st
}

func (w *BreakpointWidget) move(delta int) {
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

// syncSelectedFromViewport moves the bold blue selection to the mouse-clicked row.
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

func (w *BreakpointWidget) notifyChange() {
	if w.OnChange != nil {
		w.OnChange()
	}
}

// toggleSelected: enabled → remove from GDB but keep row (disabled);
// disabled → re-insert into GDB and mark enabled.
func (w *BreakpointWidget) toggleSelected() {
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	it := w.items[w.selected]
	loc := breakLoc(it)
	if it.Enabled {
		if it.Number > 0 {
			w.sendMI(fmt.Sprintf("-break-delete %d", it.Number))
		} else {
			w.sendMI("clear " + loc)
		}
		it.Enabled = false
		it.Number = 0
		w.items[w.selected] = it
		w.rebuild()
		w.notifyChange()
		return
	}
	w.sendMI("break " + loc)
	it.Enabled = true
	w.items[w.selected] = it
	w.rebuild()
	w.notifyChange()
}

func (w *BreakpointWidget) deleteSelected() {
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	it := w.items[w.selected]
	if it.Enabled {
		if it.Number > 0 {
			w.sendMI(fmt.Sprintf("-break-delete %d", it.Number))
		} else {
			w.sendMI("clear " + breakLoc(it))
		}
	}
	w.items = append(w.items[:w.selected], w.items[w.selected+1:]...)
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
	w.notifyChange()
}

func breakLoc(it mcp.BreakInfo) string {
	return fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
}

func (w *BreakpointWidget) sendMI(cmd string) {
	sendGdbCmd(w.sess, w.state, cmd)
}

// MergeFromGDB syncs live GDB breakpoints into the internal list without
// dropping locally disabled rows (those are intentionally absent from GDB).
func (w *BreakpointWidget) MergeFromGDB(gdbItems []mcp.BreakInfo) {
	keyOf := func(it mcp.BreakInfo) string {
		return fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
	}
	gdbByKey := make(map[string]mcp.BreakInfo, len(gdbItems))
	for _, g := range gdbItems {
		g.Enabled = true
		gdbByKey[keyOf(g)] = g
	}

	placed := make(map[string]bool)
	out := make([]mcp.BreakInfo, 0, len(w.items)+len(gdbItems))
	for _, local := range w.items {
		k := keyOf(local)
		if g, ok := gdbByKey[k]; ok {
			out = append(out, g)
			placed[k] = true
			continue
		}
		if !local.Enabled {
			// Still disabled and not in GDB — keep in the widget list.
			local.Number = 0
			out = append(out, local)
		}
		// Enabled locally but missing from GDB → dropped (deleted elsewhere).
	}
	for _, g := range gdbItems {
		k := keyOf(g)
		if placed[k] {
			continue
		}
		g.Enabled = true
		out = append(out, g)
		placed[k] = true
	}
	w.items = out

	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
	w.rebuild()
	w.notifyChange()
}

// EnabledBreakInfos returns only breakpoints currently active in GDB.
func (w *BreakpointWidget) EnabledBreakInfos() []mcp.BreakInfo {
	var out []mcp.BreakInfo
	for _, it := range w.items {
		if it.Enabled {
			out = append(out, it)
		}
	}
	return out
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
		num := "-"
		if it.Number > 0 {
			num = fmt.Sprintf("%d", it.Number)
		}
		loc := fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
		w.buf.AppendLine(fmt.Sprintf("%s  %s  %s", num, en, loc))
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
		w.viewport.HandleEvent(e)
		w.syncSelectedFromViewport()
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

func (w *BreakpointWidget) Items() []mcp.BreakInfo {
	return append([]mcp.BreakInfo(nil), w.items...)
}

func (w *BreakpointWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

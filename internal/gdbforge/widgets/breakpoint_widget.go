package widgets

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

// BreakpointWidget is a view of the shared breakpoint model.
// The app owns Merge/Toggle/Delete and GDB sends; this widget only paints
// SetItems and publishes intents on the event bus.
//
//	j/k or Up/Down — move selection and ActivateBreakpoint (browse Code, keep BP focus)
//	wheel / click — same (browse only; do not steal focus)
//	Enter — ActivateBreakpoint then FocusCode (status line → Code)
//	e — ToggleBreakpoint(selected)
//	d — DeleteBreakpoint(selected)
type BreakpointWidget struct {
	*termforge.TableWidget
	state *debugstate.State

	items []models.BreakInfo

	mouseDown bool
}

func NewBreakpointWidget() *BreakpointWidget {
	tw := termforge.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Breakpoints"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumnWidth("#", 3)
	tbl.AddColumn("En")
	tbl.AddColumn("Loc")

	w := &BreakpointWidget{TableWidget: tw}
	tw.SetRowStyleFunc(w.rowStyle)
	tw.SetFill(w.fillTable)
	w.initKeyBindings()
	return w
}

func (w *BreakpointWidget) SetAppState(st *debugstate.State) { w.state = st }

func (w *BreakpointWidget) SetItems(items []models.BreakInfo) {
	w.items = append([]models.BreakInfo(nil), items...)
	w.ResetHorizontalPan()
	w.SetSelectedRow(w.SelectedRow()) // clamp to the new row count
	w.EnsureRowVisible()
}

func (w *BreakpointWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected(false) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected(false) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.PanLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected(true) }, "<Enter>", "<C-m>")
	w.BindKeyFunc("toggle", func(args ...any) {
		if len(w.items) > 0 {
			w.Publish(events.BreakpointToggleMsg{Index: w.SelectedRow()})
		}
	}, "e")
	w.BindKeyFunc("delete", func(args ...any) {
		if len(w.items) > 0 {
			w.Publish(events.BreakpointDeleteMsg{Index: w.SelectedRow()})
		}
	}, "d")
}

func (w *BreakpointWidget) rowStyle(lineIdx int) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(w.state.MutedColor())
	}
	if lineIdx == w.SelectedRow() {
		bg := w.state.MarkDimColor()
		if w.Focused() {
			bg = w.state.MarkColor()
		}
		if lineIdx >= 0 && lineIdx < len(w.items) && w.atProgramPoint(w.items[lineIdx]) {
			bg = w.state.StackBreakColor()
		}
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	if lineIdx < 0 || lineIdx >= len(w.items) {
		return st
	}
	it := w.items[lineIdx]
	if w.atProgramPoint(it) {
		bg := w.state.StackBreakColor()
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	bg := breakGutterColor(models.BreakGutter{
		Enabled:   it.Enabled,
		Condition: it.Condition,
	}, w.state)
	return st.Background(bg).Foreground(platform.ContrastColor(bg)).Bold(true)
}

func (w *BreakpointWidget) atProgramPoint(it models.BreakInfo) bool {
	if w.state == nil {
		return false
	}
	return sameSourceLoc(it.File, it.Line, w.state.StopFile(), w.state.StopLine())
}

func (w *BreakpointWidget) move(delta int) {
	if len(w.items) == 0 {
		return
	}
	w.MoveSelection(delta)
}

func (w *BreakpointWidget) activateSelected(commitFocus bool) {
	if len(w.items) == 0 {
		return
	}
	row := w.SelectedRow()
	if row < 0 || row >= len(w.items) {
		return
	}
	w.Publish(events.BreakpointActivateMsg{BP: w.items[row], FocusCode: commitFocus})
}

func (w *BreakpointWidget) formatCols(it models.BreakInfo) (num, en, loc string) {
	en = "n"
	if it.Enabled {
		en = "y"
	}
	num = "  -"
	if it.Number > 0 {
		num = fmt.Sprintf("%3d", it.Number)
	}
	loc = "?"
	switch {
	case it.File != "" && it.Line > 0:
		loc = fmt.Sprintf("%s:%d", it.File, it.Line)
	case it.Addr != "":
		loc = "*" + it.Addr
	}
	if it.Conditional() {
		loc = fmt.Sprintf("%s  if %s", loc, it.Condition)
	}
	return num, en, loc
}

func (w *BreakpointWidget) fillTable(t *termforge.Table) {
	if len(w.items) == 0 {
		t.AddRow("no breakpoints")
		return
	}
	for _, it := range w.items {
		num, en, loc := w.formatCols(it)
		t.AddRow(num, en, loc)
	}
}

func (w *BreakpointWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		handleListMouse(w.TableWidget, &w.mouseDown, e, w.move, func() { w.activateSelected(false) })
	case *tcell.EventKey:
		w.HandleFocusKey(e)
	}
}

func (w *BreakpointWidget) SetFocused(focused bool) {
	w.TableWidget.SetFocused(focused)
	if !focused {
		w.mouseDown = false
	}
}

func (w *BreakpointWidget) Selected() int { return w.SelectedRow() }

func (w *BreakpointWidget) SelectIndex(i int) {
	w.SetSelectedRow(i)
	w.EnsureRowVisible()
}

func (w *BreakpointWidget) LinesForTest() []string {
	tbl := w.Table()
	tbl.ClearRows()
	w.fillTable(tbl)
	if len(w.items) == 0 {
		return []string{"no breakpoints"}
	}
	out := make([]string, tbl.NumRows())
	for i := 0; i < tbl.NumRows(); i++ {
		out[i] = tbl.RowDisplayLine(i)
	}
	return out
}

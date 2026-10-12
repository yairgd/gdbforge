package widgets

import (
	"fmt"
	"path/filepath"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

// CallStackWidget shows GDB stack frames.
//
//	j/k or Up/Down — move selection and ActivateCallStack (browse Code, keep stack focus)
//	wheel / click — same (browse only; do not steal focus)
//	Enter — ActivateCallStack then FocusCode (status line → Code)
type CallStackWidget struct {
	*termforge.TableWidget
	state *debugstate.State

	items []models.StackFrame

	mouseDown    bool
	lastActLevel int
	lastActTime  time.Time
}

func NewCallStackWidget() *CallStackWidget {
	tw := termforge.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Call Stack"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("#")
	tbl.AddColumn("Func")
	tbl.AddColumn("Loc")

	w := &CallStackWidget{TableWidget: tw}
	tw.SetRowStyleFunc(w.rowStyle)
	tw.SetFill(w.fillTable)
	w.initKeyBindings()
	return w
}

func (w *CallStackWidget) SetAppState(st *debugstate.State) { w.state = st }

func (w *CallStackWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected(false) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected(false) }, "<Down>", "j")
	w.BindKeyFunc("page-up", func(args ...any) { w.move(-w.PageRows()); w.activateSelected(false) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.move(w.PageRows()); w.activateSelected(false) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.moveTo(0); w.activateSelected(false) }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.moveTo(len(w.items) - 1); w.activateSelected(false) }, "<End>", "G")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.PanLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected(true) }, "<Enter>", "<C-m>")
}

func (w *CallStackWidget) moveTo(idx int) {
	w.SetSelectedRow(idx)
	w.EnsureRowVisible()
}

func (w *CallStackWidget) rowStyle(lineIdx int) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(w.state.MutedColor())
	}
	if lineIdx == w.SelectedRow() {
		bg := w.state.MarkDimColor()
		if w.Focused() {
			bg = w.state.MarkColor()
		}
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	if w.isFrameZero(lineIdx) && w.atProgramPoint(lineIdx) {
		bg := w.state.StackBreakColor()
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (w *CallStackWidget) atProgramPoint(lineIdx int) bool {
	if w.state == nil || lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	it := w.items[lineIdx]
	return sameSourceLoc(it.File, it.Line, w.state.StopFile(), w.state.StopLine())
}

func (w *CallStackWidget) isFrameZero(lineIdx int) bool {
	if lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	return w.items[lineIdx].Level == 0
}

func (w *CallStackWidget) move(delta int) {
	if len(w.items) == 0 {
		return
	}
	w.MoveSelection(delta)
}

func (w *CallStackWidget) activateSelected(commitFocus bool) {
	if len(w.items) == 0 {
		return
	}
	row := w.SelectedRow()
	if row < 0 || row >= len(w.items) {
		return
	}
	fr := w.items[row]
	now := time.Now()
	if fr.Level != w.lastActLevel || now.Sub(w.lastActTime) >= 300*time.Millisecond {
		w.lastActLevel = fr.Level
		w.lastActTime = now
		w.Publish(events.CallStackActivateMsg{Frame: fr, FocusCode: commitFocus})
	} else if commitFocus {
		w.Publish(events.FocusCodeMsg{})
	}
}

func (w *CallStackWidget) SetItems(items []models.StackFrame) {
	prevLevel := -1
	row := w.SelectedRow()
	if row >= 0 && row < len(w.items) {
		prevLevel = w.items[row].Level
	}
	w.items = append([]models.StackFrame(nil), items...)
	w.ResetHorizontalPan()
	sel := 0
	if prevLevel >= 0 {
		for i, it := range w.items {
			if it.Level == prevLevel {
				sel = i
				break
			}
		}
	}
	w.SetSelectedRow(sel)
	w.EnsureRowVisible()
}

func (w *CallStackWidget) SelectLevel(level int) {
	for i, it := range w.items {
		if it.Level == level {
			w.SetSelectedRow(i)
			w.EnsureRowVisible()
			return
		}
	}
}

func (w *CallStackWidget) fillTable(t *termforge.Table) {
	if len(w.items) == 0 {
		t.AddRow("no frames")
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
		t.AddRow(fmt.Sprintf("%d", it.Level), fn, loc)
	}
}

func (w *CallStackWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		handleListMouse(w.TableWidget, &w.mouseDown, e, w.move, func() { w.activateSelected(false) })
	case *tcell.EventKey:
		w.HandleFocusKey(e)
	}
}

func (w *CallStackWidget) SetFocused(focused bool) {
	w.TableWidget.SetFocused(focused)
	if !focused {
		w.mouseDown = false
	}
}

func (w *CallStackWidget) Selected() int { return w.SelectedRow() }

func (w *CallStackWidget) SelectedFrame() (models.StackFrame, bool) {
	row := w.SelectedRow()
	if row < 0 || row >= len(w.items) {
		return models.StackFrame{}, false
	}
	return w.items[row], true
}

func (w *CallStackWidget) Items() []models.StackFrame {
	return append([]models.StackFrame(nil), w.items...)
}

func (w *CallStackWidget) LinesForTest() []string {
	tbl := w.Table()
	tbl.ClearRows()
	w.fillTable(tbl)
	if len(w.items) == 0 {
		return []string{"no frames"}
	}
	out := make([]string, tbl.NumRows())
	for i := 0; i < tbl.NumRows(); i++ {
		out[i] = tbl.RowDisplayLine(i)
	}
	return out
}

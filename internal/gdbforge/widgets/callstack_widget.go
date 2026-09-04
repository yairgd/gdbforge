package widgets

import (
	"fmt"
	"path/filepath"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// CallStackWidget shows GDB stack frames.
//
//	j/k or Up/Down — move selection and ActivateCallStack (browse Code, keep stack focus)
//	wheel / click — same (browse only; do not steal focus)
//	Enter — ActivateCallStack then FocusCode (status line → Code)
type CallStackWidget struct {
	*termui.TableWidget
	state *debugstate.State

	items []models.StackFrame

	mouseDown     bool
	pressOnRow    bool
	pressSelected int
	lastActLevel  int
	lastActTime   time.Time
}

func NewCallStackWidget() *CallStackWidget {
	tw := termui.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Call Stack"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("#")
	tbl.AddColumn("Func")
	tbl.AddColumn("Loc")

	w := &CallStackWidget{TableWidget: tw}
	tw.SetRowStyleFunc(func(row int) tcell.Style { return w.rowStyle(row, "") })
	tw.SetOnSearchJump(func(row int) { tw.SetSelectedRow(row) })
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
	n := len(w.items)
	if n == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	w.SetSelectedRow(idx)
	w.EnsureRowVisible()
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

func (w *CallStackWidget) rowStyle(lineIdx int, _ string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.items) == 0 {
		return st.Foreground(w.mutedColor())
	}
	if lineIdx == w.SelectedRow() {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
		}
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	if w.isFrameZero(lineIdx) && w.atProgramPoint(lineIdx) {
		bg := w.stackBreakColor()
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

func (w *CallStackWidget) syncSelectedFromViewport() {
	n := len(w.items)
	if n == 0 {
		return
	}
	row := w.SelectedRow()
	if row < 0 {
		row = 0
	}
	if row >= n {
		row = n - 1
	}
	w.SetSelectedRow(row)
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
	w.RectViewport().Origin.X = 0
	sel := 0
	if prevLevel >= 0 {
		for i, it := range w.items {
			if it.Level == prevLevel {
				sel = i
				break
			}
		}
	}
	if sel >= len(w.items) && len(w.items) > 0 {
		sel = len(w.items) - 1
	}
	if sel < 0 {
		sel = 0
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

func (w *CallStackWidget) fillTable(t *termui.Table) {
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
		if w.TryDoubleClickWord(e) {
			return
		}
		btns := e.Buttons()
		if btns&tcell.WheelUp != 0 {
			w.move(-1)
			w.activateSelected(false)
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.move(1)
			w.activateSelected(false)
			return
		}
		mx, my := e.Position()
		hitRow, onRow := w.HitDataRow(mx, my)
		if btns&tcell.ButtonPrimary != 0 {
			if onRow {
				w.SetSelectedRow(hitRow)
				if !w.mouseDown {
					w.mouseDown = true
					w.pressOnRow = true
					w.pressSelected = w.SelectedRow()
				}
			} else if !w.mouseDown {
				w.pressOnRow = false
			}
			return
		}
		if w.mouseDown {
			w.mouseDown = false
			if onRow {
				w.SetSelectedRow(hitRow)
			}
			if w.pressOnRow {
				w.activateSelected(false)
			}
			w.pressOnRow = false
		}
	case *tcell.EventKey:
		w.HandleFocusKey(e)
	}
}

func (w *CallStackWidget) SetFocused(focused bool) {
	w.TableWidget.SetFocused(focused)
	if !focused {
		w.mouseDown = false
		w.pressOnRow = false
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

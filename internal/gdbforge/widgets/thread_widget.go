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

// ThreadWidget shows GDB threads.
//
//	j/k or Up/Down — move selection and ActivateThread (like Enter)
//	wheel — same as j/k (Code / GDB follow the selected thread)
//	Enter / click — ActivateThread
type ThreadWidget struct {
	*termui.TableWidget
	state *debugstate.State

	items []models.ThreadInfo

	mouseDown     bool
	pressSelected int
	lastActID     string
	lastActTime   time.Time
}

func NewThreadWidget() *ThreadWidget {
	tw := termui.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Threads"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("ID")
	tbl.AddColumn("State")
	tbl.AddColumn("Loc")

	w := &ThreadWidget{TableWidget: tw}
	tw.SetRowStyleFunc(func(row int) tcell.Style { return w.rowStyle(row, "") })
	tw.SetOnSearchJump(func(row int) { tw.SetSelectedRow(row) })
	tw.SetFill(w.fillTable)
	w.initKeyBindings()
	return w
}

func (w *ThreadWidget) SetAppState(st *debugstate.State) { w.state = st }

func (w *ThreadWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1); w.activateSelected() }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1); w.activateSelected() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.PanLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("activate", func(args ...any) { w.activateSelected() }, "<Enter>", "<C-m>")
}

func (w *ThreadWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (w *ThreadWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (w *ThreadWidget) stackBreakColor() tcell.Color {
	if w.state != nil {
		return w.state.StackBreakColor()
	}
	return platform.DefaultStackBreakColor
}

func (w *ThreadWidget) mutedColor() tcell.Color {
	if w.state != nil {
		return w.state.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (w *ThreadWidget) rowStyle(lineIdx int, _ string) tcell.Style {
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
	if w.isCurrentThread(lineIdx) && w.atProgramPoint(lineIdx) {
		bg := w.stackBreakColor()
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (w *ThreadWidget) atProgramPoint(lineIdx int) bool {
	if w.state == nil || lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	it := w.items[lineIdx]
	return sameSourceLoc(it.File, it.Line, w.state.StopFile(), w.state.StopLine())
}

func (w *ThreadWidget) isCurrentThread(lineIdx int) bool {
	if lineIdx < 0 || lineIdx >= len(w.items) {
		return false
	}
	return w.items[lineIdx].Current
}

func (w *ThreadWidget) move(delta int) {
	if len(w.items) == 0 {
		return
	}
	w.MoveSelection(delta)
}

func (w *ThreadWidget) syncSelectedFromViewport() {
	w.clampSelectedToItems()
}

func (w *ThreadWidget) clampSelectedToItems() {
	n := len(w.items)
	if n == 0 {
		w.SetSelectedRow(0)
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

func (w *ThreadWidget) activateSelected() {
	if len(w.items) == 0 {
		return
	}
	row := w.SelectedRow()
	if row < 0 || row >= len(w.items) {
		return
	}
	th := w.items[row]
	now := time.Now()
	if th.ID == w.lastActID && now.Sub(w.lastActTime) < 300*time.Millisecond {
		return
	}
	w.lastActID = th.ID
	w.lastActTime = now
	w.Publish(events.ThreadActivateMsg{Thread: th})
}

func (w *ThreadWidget) SetItems(items []models.ThreadInfo) {
	prevID := ""
	row := w.SelectedRow()
	if row >= 0 && row < len(w.items) {
		prevID = w.items[row].ID
	}
	w.items = append([]models.ThreadInfo(nil), items...)
	w.RectViewport().Origin.X = 0
	sel := 0
	if prevID != "" {
		for i, it := range w.items {
			if it.ID == prevID {
				sel = i
				break
			}
		}
	} else {
		for i, it := range w.items {
			if it.Current {
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

func (w *ThreadWidget) fillTable(t *termui.Table) {
	if len(w.items) == 0 {
		t.AddRow("no threads")
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
		t.AddRow(it.ID, state, loc)
	}
}

func (w *ThreadWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if w.TryDoubleClickWord(e) {
			return
		}
		btns := e.Buttons()
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
		mx, my := e.Position()
		hitRow, onRow := w.HitDataRow(mx, my)
		if btns&tcell.ButtonPrimary != 0 {
			if onRow {
				w.SetSelectedRow(hitRow)
				if !w.mouseDown {
					w.mouseDown = true
					w.pressSelected = w.SelectedRow()
				}
			}
			return
		}
		if w.mouseDown {
			w.mouseDown = false
			if onRow {
				w.SetSelectedRow(hitRow)
			}
			if w.pressSelected != w.SelectedRow() {
				w.activateSelected()
			} else {
				w.activateSelected()
			}
		}
	case *tcell.EventKey:
		w.HandleFocusKey(e)
	}
}

func (w *ThreadWidget) SetFocused(focused bool) {
	w.TableWidget.SetFocused(focused)
	if !focused {
		w.mouseDown = false
	}
}

func (w *ThreadWidget) Selected() int { return w.SelectedRow() }

func (w *ThreadWidget) Items() []models.ThreadInfo {
	return append([]models.ThreadInfo(nil), w.items...)
}

func (w *ThreadWidget) LinesForTest() []string {
	tbl := w.Table()
	tbl.ClearRows()
	w.fillTable(tbl)
	if len(w.items) == 0 {
		return []string{"no threads"}
	}
	out := make([]string, tbl.NumRows())
	for i := 0; i < tbl.NumRows(); i++ {
		out[i] = tbl.RowDisplayLine(i)
	}
	return out
}

func (w *ThreadWidget) ViewportLeftForTest() int {
	return w.RectViewport().Origin.X
}

package widgets

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
)

// FileListWidget shows GDB project source files (j/k selection; Enter opens).
// Mouse: first click selects (blue mark); second click on the marked row opens;
// the wheel moves the selection.
type FileListWidget struct {
	*termforge.TableWidget
	state *debugstate.State

	paths []string
}

func NewFileListWidget() *FileListWidget {
	tw := termforge.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Files"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("#")
	tbl.AddColumn("File")

	w := &FileListWidget{TableWidget: tw}
	tw.SetRowStyleFunc(w.rowStyle)
	tw.SetFill(w.fillTable)
	w.initKeyBindings()
	return w
}

func (w *FileListWidget) SetAppState(st *debugstate.State) { w.state = st }

func (w *FileListWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
	w.BindKeyFunc("page-up", func(args ...any) { w.move(-w.PageRows()) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.move(w.PageRows()) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.moveTo(0) }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.moveTo(len(w.paths) - 1) }, "<End>", "G")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.PanLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("open", func(args ...any) { w.openSelected() }, "<Enter>", "<C-m>")
}

func (w *FileListWidget) rowStyle(lineIdx int) tcell.Style {
	st := tcell.StyleDefault
	if len(w.paths) == 0 {
		return st.Foreground(w.state.MutedColor())
	}
	if lineIdx == w.SelectedRow() {
		bg := w.state.MarkDimColor()
		if w.Focused() {
			bg = w.state.MarkColor()
		}
		return st.Bold(true).Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (w *FileListWidget) move(delta int) {
	if len(w.paths) == 0 {
		return
	}
	w.MoveSelection(delta)
}

func (w *FileListWidget) moveTo(row int) {
	w.SetSelectedRow(row)
	w.EnsureRowVisible()
}

func (w *FileListWidget) openSelected() {
	if len(w.paths) == 0 {
		return
	}
	row := w.SelectedRow()
	if row < 0 || row >= len(w.paths) {
		return
	}
	w.Publish(events.OpenSourceMsg{Path: w.paths[row]})
}

func (w *FileListWidget) SetItems(paths []string) {
	w.paths = append([]string(nil), paths...)
	w.ResetHorizontalPan()
	w.SetSelectedRow(w.SelectedRow()) // clamp to the new row count
	w.EnsureRowVisible()
}

func (w *FileListWidget) fillTable(t *termforge.Table) {
	if len(w.paths) == 0 {
		t.AddRow("no files")
		return
	}
	for i, p := range w.paths {
		t.AddRow(fmt.Sprintf("%d", i+1), p)
	}
}

func (w *FileListWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		if w.TryDoubleClickWord(e) {
			return
		}
		btns := e.Buttons()
		if btns&tcell.WheelUp != 0 {
			w.move(-1)
			return
		}
		if btns&tcell.WheelDown != 0 {
			w.move(1)
			return
		}
		if btns&tcell.ButtonPrimary == 0 {
			return
		}
		mx, my := e.Position()
		hitRow, onRow := w.HitDataRow(mx, my)
		if !onRow {
			return
		}
		prev := w.SelectedRow()
		w.SetSelectedRow(hitRow)
		if hitRow == prev {
			w.openSelected()
		}
	case *tcell.EventKey:
		w.HandleFocusKey(e)
	}
}

func (w *FileListWidget) Paths() []string {
	return append([]string(nil), w.paths...)
}

func (w *FileListWidget) Selected() int { return w.SelectedRow() }

func (w *FileListWidget) LinesForTest() []string {
	tbl := w.Table()
	tbl.ClearRows()
	w.fillTable(tbl)
	if len(w.paths) == 0 {
		return []string{"no files"}
	}
	out := make([]string, tbl.NumRows())
	for i := 0; i < tbl.NumRows(); i++ {
		out[i] = tbl.RowDisplayLine(i)
	}
	return out
}

package widgets

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/events"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// FileListWidget shows GDB project source files (j/k selection; Enter opens).
// Mouse: first click selects (blue mark); second click on the marked row opens.
type FileListWidget struct {
	*termui.TableWidget
	state *debugstate.State

	paths []string
}

func NewFileListWidget() *FileListWidget {
	tw := termui.NewTableWidget(platform.NewAppContext())
	tw.PaneName = "Files"
	tbl := tw.Table()
	tbl.SetShowHeader(false)
	tbl.SetGutter(2)
	tbl.AddColumn("#")
	tbl.AddColumn("File")

	w := &FileListWidget{TableWidget: tw}
	tw.SetRowStyleFunc(func(row int) tcell.Style { return w.rowStyle(row, "") })
	tw.SetOnSearchJump(func(row int) { tw.SetSelectedRow(row) })
	tw.SetFill(w.fillTable)
	w.initKeyBindings()
	return w
}

func (w *FileListWidget) SetAppState(st *debugstate.State) { w.state = st }

func (w *FileListWidget) initKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.move(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.move(1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.PanLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.PanRight() }, "<Right>")
	w.BindKeyFunc("open", func(args ...any) { w.openSelected() }, "<Enter>", "<C-m>")
}

func (w *FileListWidget) markColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (w *FileListWidget) markDimColor() tcell.Color {
	if w.state != nil {
		return w.state.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (w *FileListWidget) mutedColor() tcell.Color {
	if w.state != nil {
		return w.state.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (w *FileListWidget) rowStyle(lineIdx int, _ string) tcell.Style {
	st := tcell.StyleDefault
	if len(w.paths) == 0 {
		return st.Foreground(w.mutedColor())
	}
	if lineIdx == w.SelectedRow() {
		bg := w.markDimColor()
		if w.Focused() {
			bg = w.markColor()
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
	row := w.SelectedRow()
	if row >= len(w.paths) && len(w.paths) > 0 {
		row = len(w.paths) - 1
	}
	if row < 0 {
		row = 0
	}
	w.RectViewport().Origin.X = 0
	w.SetSelectedRow(row)
	w.EnsureRowVisible()
}

func (w *FileListWidget) fillTable(t *termui.Table) {
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
		if e.Buttons()&tcell.ButtonPrimary == 0 {
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

func (w *FileListWidget) SetFocused(focused bool) {
	w.TableWidget.SetFocused(focused)
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

package termui

import (
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// TableFillFunc repopulates table rows at Draw time (e.g. from external model data).
type TableFillFunc func(t *Table)

// TableWidget is a columnar table view with RectViewport panning, row selection,
// /search, copy, and mouse hit-testing.
type TableWidget struct {
	BaseWidget
	table *Table
	rv    *RectViewport
	buf   *CellBuffer
	fill  TableFillFunc

	selectedRow int
	rowStyleFn  func(row int) tcell.Style

	searchPattern   string
	searchCommitted string
	searchColor     tcell.Color
	searchHits      []TableSearchHit
	onSearchJump    func(row int)

	clipboard ClipboardIO

	screenX int
	screenY int

	lastW int
	lastH int

	selectColor tcell.Color

	cellSel         *TableSearchHit
	cellSelText     string
	suppressCellClk bool
	clickCount      int
	lastClickTime   time.Time
	lastClickRow    int
	lastClickCol    int
}

var (
	_ Widget          = (*TableWidget)(nil)
	_ FocusKeyHandler = (*TableWidget)(nil)
	_ Focusable       = (*TableWidget)(nil)
	_ SearchHost      = (*TableWidget)(nil)
)

func NewTableWidget(ctx platform.AppContext) *TableWidget {
	return &TableWidget{
		BaseWidget:  NewBaseWidget(ctx),
		table:       NewTable(),
		rv:          NewRectViewport(),
		selectColor: tcell.ColorDarkSlateBlue,
	}
}

func (w *TableWidget) Table() *Table { return w.table }

func (w *TableWidget) SetFill(fn TableFillFunc) { w.fill = fn }

func (w *TableWidget) RectViewport() *RectViewport { return w.rv }

func (w *TableWidget) SetRowStyleFunc(fn func(row int) tcell.Style) { w.rowStyleFn = fn }

func (w *TableWidget) SetOnSearchJump(fn func(row int)) { w.onSearchJump = fn }

func (w *TableWidget) SelectedRow() int { return w.selectedRow }

func (w *TableWidget) PanLeft()  { w.pan(-1, 0) }
func (w *TableWidget) PanRight() { w.pan(1, 0) }

func (w *TableWidget) clampSelected() {
	n := w.table.NumRows()
	if n == 0 {
		w.selectedRow = 0
		return
	}
	if w.selectedRow < 0 {
		w.selectedRow = 0
	}
	if w.selectedRow >= n {
		w.selectedRow = n - 1
	}
}

// InitPanKeyBindings wires Up/Down/Left/Right (and page/home/end) for pan-only
// demos. List panes override with selection-aware bindings.
func (w *TableWidget) InitPanKeyBindings() {
	w.BindKeyFunc("scroll-up", func(args ...any) { w.pan(0, -1) }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.pan(0, 1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.pan(-1, 0) }, "<Left>", "h")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.pan(1, 0) }, "<Right>", "l")
	w.BindKeyFunc("page-up", func(args ...any) { w.pan(0, -w.PageRows()) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.pan(0, w.PageRows()) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.scrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.scrollEnd() }, "<End>", "G")
}

// InitSelectionKeyBindings wires Up/Down for row selection and Left/Right for pan.
func (w *TableWidget) InitSelectionKeyBindings() {
	w.BindKeyFunc("up", func(args ...any) { w.MoveSelection(-1) }, "<Up>", "k")
	w.BindKeyFunc("down", func(args ...any) { w.MoveSelection(1) }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.pan(-1, 0) }, "<Left>", "h")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.pan(1, 0) }, "<Right>", "l")
	w.BindKeyFunc("page-up", func(args ...any) {
		w.MoveSelection(-w.PageRows())
	}, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) {
		w.MoveSelection(w.PageRows())
	}, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.SetSelectedRow(0); w.EnsureRowVisible() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) {
		w.SetSelectedRow(w.table.NumRows() - 1)
		w.EnsureRowVisible()
	}, "<End>", "G")
}

func (w *TableWidget) MoveSelection(delta int) {
	w.syncFill()
	n := w.table.NumRows()
	if n == 0 {
		return
	}
	w.selectedRow = (w.selectedRow+delta%n + n) % n
	w.ensureRowVisible()
}

func (w *TableWidget) EnsureRowVisible() {
	w.syncFill()
	dataH := w.dataViewportH(w.lastH)
	w.rv.EnsureRowVisible(w.selectedRow, dataH)
	w.clampView()
}

func (w *TableWidget) ensureRowVisible() { w.EnsureRowVisible() }

func (w *TableWidget) SetSelectedRow(row int) {
	w.syncFill()
	w.selectedRow = row
	w.clampSelected()
}

func (w *TableWidget) PageRows() int {
	h := w.dataViewportH(w.lastH)
	if h < 1 {
		return 10
	}
	return h
}

func (w *TableWidget) dataViewportH(windowH int) int {
	if windowH <= 0 {
		return 0
	}
	lay := w.table.Layout()
	h := windowH - lay.stickyRows
	if h < 0 {
		return 0
	}
	return h
}

func (w *TableWidget) pan(dx, dy int) {
	w.rv.Pan(dx, dy)
	w.clampView()
}

func (w *TableWidget) scrollHome() {
	w.rv.ScrollHome()
	w.clampView()
}

func (w *TableWidget) scrollEnd() {
	w.rv.ScrollEnd(w.dataViewportH(w.lastH))
	w.clampView()
}

func (w *TableWidget) clampView() {
	w.rv.Clamp(w.lastW, w.dataViewportH(w.lastH))
}

func (w *TableWidget) syncFill() {
	if w.fill != nil {
		w.table.ClearRows()
		w.fill(w.table)
		w.clampSelected()
	}
	if w.searchPattern != "" {
		w.searchHits = RebuildTableSearch(w.table, w.searchPattern)
	} else {
		w.searchHits = nil
	}
}

func (w *TableWidget) paintState() TablePaintState {
	return TablePaintState{
		RowStyle:    w.rowStyleFn,
		SearchColor: w.searchColor,
		SearchHits:  w.searchHits,
		SelectHit:   w.cellSel,
		SelectColor: w.selectColor,
	}
}

func (w *TableWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *TableWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventKey:
		w.HandleBoundKey(e)
	case *tcell.EventMouse:
		w.handleMouse(e)
	}
}

func (w *TableWidget) handleMouse(e *tcell.EventMouse) {
	if w.TryDoubleClickWord(e) {
		return
	}
	btns := e.Buttons()
	if btns&tcell.WheelUp != 0 {
		w.MoveSelection(-1)
		return
	}
	if btns&tcell.WheelDown != 0 {
		w.MoveSelection(1)
		return
	}
	mx, my := e.Position()
	row, onRow := w.HitDataRow(mx, my)
	if btns&tcell.ButtonPrimary != 0 {
		if onRow {
			w.SetSelectedRow(row)
		}
	}
}

// TryDoubleClickWord handles double-click on a cell: highlight the full cell
// text and copy it to the clipboard. Returns true when the event is consumed
// (second click of a double-click, or while holding after one).
func (w *TableWidget) TryDoubleClickWord(e *tcell.EventMouse) bool {
	if e == nil {
		return false
	}
	btns := e.Buttons()
	if btns == tcell.ButtonNone {
		w.suppressCellClk = false
		return false
	}
	if w.suppressCellClk {
		return true
	}
	if btns&tcell.ButtonPrimary == 0 {
		return false
	}
	mx, my := e.Position()
	row, col, _, ok := w.HitCell(mx, my)
	if !ok {
		w.ClearCellSelection()
		return false
	}
	w.noteCellClick(row, col, e.When())
	if w.clickCount == 2 {
		text := w.table.CellText(row, col)
		rs := []rune(text)
		if len(rs) == 0 {
			return false
		}
		w.cellSel = &TableSearchHit{Row: row, Col: col, Start: 0, End: len(rs)}
		w.cellSelText = text
		w.clipboard.copyText(text)
		w.SetSelectedRow(row)
		w.suppressCellClk = true
		return true
	}
	w.ClearCellSelection()
	return false
}

func (w *TableWidget) noteCellClick(row, col int, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	same := row == w.lastClickRow && col == w.lastClickCol
	if same && when.Sub(w.lastClickTime) <= clickMultiTimeoutMs*time.Millisecond {
		w.clickCount++
		if w.clickCount > 3 {
			w.clickCount = 1
		}
	} else {
		w.clickCount = 1
	}
	w.lastClickTime = when
	w.lastClickRow = row
	w.lastClickCol = col
}

// ClearCellSelection drops the highlighted cell text (e.g. before a new single click).
func (w *TableWidget) ClearCellSelection() {
	w.cellSel = nil
	w.cellSelText = ""
}

// HitDataRow maps screen coordinates to a data row index.
func (w *TableWidget) HitDataRow(mx, my int) (row int, onRow bool) {
	w.syncFill()
	lx := mx - w.screenX
	ly := my - w.screenY
	if lx < 0 || ly < 0 || lx >= w.lastW || ly >= w.lastH {
		return -1, false
	}
	lay := w.table.Layout()
	if ly < lay.stickyRows {
		return -1, false
	}
	rel := ly - lay.stickyRows
	dataH := w.dataViewportH(w.lastH)
	if rel >= dataH {
		return -1, false
	}
	dr := w.rv.Origin.Y + rel
	if dr < 0 || dr >= w.table.NumRows() {
		return -1, false
	}
	return dr, true
}

func (w *TableWidget) SetClipboard(io ClipboardIO) { w.clipboard = io }

func (w *TableWidget) CopySelection() {
	if w.clipboard.Copy == nil {
		return
	}
	if w.cellSelText != "" {
		w.clipboard.copyText(w.cellSelText)
		return
	}
	text := w.table.RowDisplayLine(w.selectedRow)
	w.clipboard.copyText(text)
}

func (w *TableWidget) SelectedText() string {
	if w.cellSelText != "" {
		return w.cellSelText
	}
	return w.table.RowDisplayLine(w.selectedRow)
}

func (w *TableWidget) HasSelection() bool { return w.cellSel != nil }

func (w *TableWidget) Draw(c Canvas) {
	w.syncFill()
	w.lastW = c.W()
	w.lastH = c.H()
	w.screenX = c.ScreenX(0)
	w.screenY = c.ScreenY(0)
	if w.buf == nil {
		w.buf = NewCellBuffer(w.lastW, w.lastH)
	}
	w.table.PaintVisible(w.buf, w.rv, w.lastW, w.lastH, w.paintState())
	w.buf.BlitTo(c, 0, 0)
}

func (w *TableWidget) DrawStatusLine(c Canvas, active bool) {
	w.BaseWidget.DrawStatusLine(c, active)
}

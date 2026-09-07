package termui

import (
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
)

const defaultTableGutter = 1

// TableColumn describes one table column.
type TableColumn struct {
	Name     string
	Width    int // 0 = auto from header + cells
	MinWidth int
}

// TableCell is one cell's text and optional style.
type TableCell struct {
	Text  string
	Style tcell.Style
}

// Table holds column definitions and row data (layout + paint input).
type Table struct {
	title      string
	showTitle  bool
	showHeader bool
	columns    []TableColumn
	rows       [][]TableCell

	titleStyle  tcell.Style
	headerStyle tcell.Style
	rowStyle    tcell.Style
	gutter      int
}

func NewTable() *Table {
	return &Table{
		gutter:      defaultTableGutter,
		titleStyle:  tcell.StyleDefault.Bold(true),
		headerStyle: tcell.StyleDefault.Bold(true),
		rowStyle:    tcell.StyleDefault,
	}
}

func (t *Table) SetTitle(s string)       { t.title = s }
func (t *Table) SetShowTitle(show bool)  { t.showTitle = show }
func (t *Table) SetShowHeader(show bool) { t.showHeader = show }

func (t *Table) SetTitleStyle(st tcell.Style)  { t.titleStyle = st }
func (t *Table) SetHeaderStyle(st tcell.Style) { t.headerStyle = st }
func (t *Table) SetRowStyle(st tcell.Style)    { t.rowStyle = st }

func (t *Table) SetGutter(n int) {
	if n < 0 {
		n = 0
	}
	t.gutter = n
}

func (t *Table) Title() string    { return t.title }
func (t *Table) ShowTitle() bool  { return t.showTitle }
func (t *Table) ShowHeader() bool { return t.showHeader }

func (t *Table) AddColumn(name string) int {
	return t.AddColumnWidth(name, 0)
}

func (t *Table) AddColumnWidth(name string, width int) int {
	t.columns = append(t.columns, TableColumn{Name: name, Width: width})
	return len(t.columns) - 1
}

func (t *Table) NumCols() int { return len(t.columns) }
func (t *Table) NumRows() int { return len(t.rows) }

func (t *Table) ClearRows() {
	t.rows = t.rows[:0]
}

func (t *Table) AddRow(values ...string) {
	cells := make([]TableCell, len(values))
	for i, v := range values {
		cells[i] = TableCell{Text: v}
	}
	t.AddRowCells(cells...)
}

func (t *Table) AddRowCells(cells ...TableCell) {
	row := make([]TableCell, len(t.columns))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
	}
	t.rows = append(t.rows, row)
}

// tableLayout is computed column widths and content dimensions.
type tableLayout struct {
	colWidths  []int
	contentW   int
	stickyRows int // title + header rows pinned at window top
	dataRows   int
	hasTitle   bool
	hasHeader  bool
}

func (t *Table) Layout() tableLayout {
	nc := len(t.columns)
	lay := tableLayout{
		colWidths: make([]int, nc),
		hasTitle:  t.showTitle && t.title != "",
		hasHeader: t.showHeader && nc > 0,
		dataRows:  len(t.rows),
	}
	if lay.hasTitle {
		lay.stickyRows++
	}
	if lay.hasHeader {
		lay.stickyRows++
	}

	for i, col := range t.columns {
		w := col.Width
		if w <= 0 {
			w = runeLen(col.Name)
			for _, row := range t.rows {
				if i < len(row) {
					if cw := runeLen(row[i].Text); cw > w {
						w = cw
					}
				}
			}
		}
		if col.MinWidth > w {
			w = col.MinWidth
		}
		lay.colWidths[i] = w
	}

	gutter := t.gutter
	if nc > 0 {
		sum := 0
		for _, w := range lay.colWidths {
			sum += w
		}
		if nc > 1 {
			sum += gutter * (nc - 1)
		}
		lay.contentW = sum
	}
	if lay.hasTitle {
		if tw := runeLen(t.title); tw > lay.contentW {
			lay.contentW = tw
		}
	}
	return lay
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(rs[:max-1]) + "…"
}

func mergeCellStyle(cell, row tcell.Style) tcell.Style {
	if cell != (tcell.Style{}) {
		return cell
	}
	return row
}

func (t *Table) paintContentRunes(
	buf *CellBuffer,
	wy int,
	originX int,
	windowW int,
	lay tableLayout,
	cells []TableCell,
	dataRow int,
	ps TablePaintState,
) {
	if windowW <= 0 {
		return
	}
	rowStyle := ps.rowStyle(dataRow, t.rowStyle)
	gutter := t.gutter
	xContent := 0
	for col := 0; col < len(lay.colWidths); col++ {
		if col > 0 {
			for g := 0; g < gutter; g++ {
				if xContent >= originX && xContent < originX+windowW {
					buf.Set(xContent-originX, wy, ' ', rowStyle)
				}
				xContent++
			}
		}
		cw := lay.colWidths[col]
		text := ""
		st := rowStyle
		if col < len(cells) {
			text = truncateRunes(cells[col].Text, cw)
			st = mergeCellStyle(cells[col].Style, rowStyle)
		}
		rs := []rune(text)
		for i := 0; i < cw; i++ {
			ch := ' '
			if i < len(rs) {
				ch = rs[i]
			}
			if xContent >= originX && xContent < originX+windowW {
				buf.Set(xContent-originX, wy, ch, ps.cellStyle(dataRow, col, i, st))
			}
			xContent++
		}
	}
}

func (t *Table) paintTitle(buf *CellBuffer, lay tableLayout, wy, originX, windowW int) {
	if !lay.hasTitle || windowW <= 0 {
		return
	}
	rs := []rune(t.title)
	for wx := 0; wx < windowW; wx++ {
		cx := originX + wx
		ch := ' '
		if cx >= 0 && cx < len(rs) {
			ch = rs[cx]
		}
		buf.Set(wx, wy, ch, t.titleStyle)
	}
}

func (t *Table) paintHeader(buf *CellBuffer, lay tableLayout, wy, originX, windowW int) {
	if !lay.hasHeader {
		return
	}
	cells := make([]TableCell, len(t.columns))
	for i, col := range t.columns {
		cells[i] = TableCell{Text: col.Name}
	}
	t.paintContentRunes(buf, wy, originX, windowW, lay, cells, -1, TablePaintState{RowStyle: func(int) tcell.Style { return t.headerStyle }})
}

// PaintVisible renders the visible slice into buf (window-sized).
func (t *Table) PaintVisible(buf *CellBuffer, rv *RectViewport, windowW, windowH int, ps TablePaintState) {
	if buf == nil || rv == nil {
		return
	}
	buf.EnsureSize(windowW, windowH)
	buf.Clear(tcell.StyleDefault)

	lay := t.Layout()
	dataH := windowH - lay.stickyRows
	if dataH < 0 {
		dataH = 0
	}

	rv.SetContentSize(lay.contentW, lay.dataRows)
	rv.Clamp(windowW, dataH)

	wy := 0
	if lay.hasTitle {
		t.paintTitle(buf, lay, wy, rv.Origin.X, windowW)
		wy++
	}
	if lay.hasHeader {
		t.paintHeader(buf, lay, wy, rv.Origin.X, windowW)
		wy++
	}

	for row := rv.Origin.Y; row < lay.dataRows && wy < windowH; row++ {
		var cells []TableCell
		if row >= 0 && row < len(t.rows) {
			cells = t.rows[row]
		}
		t.paintContentRunes(buf, wy, rv.Origin.X, windowW, lay, cells, row, ps)
		wy++
	}
	for ; wy < windowH; wy++ {
		for x := 0; x < windowW; x++ {
			buf.Set(x, wy, ' ', t.rowStyle)
		}
	}
}

// PaintVisibleDefault paints with no row styling or search highlights.
func (t *Table) PaintVisibleDefault(buf *CellBuffer, rv *RectViewport, windowW, windowH int) {
	t.PaintVisible(buf, rv, windowW, windowH, TablePaintState{})
}

func (t *Table) ContentOverflows(windowW, windowH int) bool {
	lay := t.Layout()
	dataH := windowH - lay.stickyRows
	if dataH < 0 {
		dataH = 0
	}
	return lay.contentW > windowW || lay.dataRows > dataH
}

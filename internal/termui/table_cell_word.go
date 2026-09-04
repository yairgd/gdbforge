package termui

// CellText returns the full text of one data cell.
func (t *Table) CellText(row, col int) string {
	if t == nil || row < 0 || row >= len(t.rows) || col < 0 || col >= len(t.columns) {
		return ""
	}
	if col < len(t.rows[row]) {
		return t.rows[row][col].Text
	}
	return ""
}

func (t *Table) Gutter() int {
	if t == nil {
		return 0
	}
	return t.gutter
}

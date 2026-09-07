package termui

import tcell "github.com/gdamore/tcell/v2"

func (w *TableWidget) SetSearchColor(c tcell.Color) { w.searchColor = c }

func (w *TableWidget) SetSearchPattern(pattern string) {
	w.searchPattern = pattern
	w.syncFill()
}

func (w *TableWidget) CommitSearch(pattern string) {
	w.searchPattern = pattern
	w.searchCommitted = pattern
	w.syncFill()
	if pattern == "" {
		return
	}
	if tableRowMatches(w.table, w.selectedRow, pattern) {
		w.jumpToSearchRow(w.selectedRow)
		return
	}
	_ = w.SearchNext()
}

func (w *TableWidget) RevertSearch() { w.searchPattern = w.searchCommitted; w.syncFill() }

func (w *TableWidget) SearchPattern() string { return w.searchPattern }

func (w *TableWidget) SearchNext() bool {
	row, ok := tableSearchJump(w.table, w.selectedRow, 1, w.searchPattern)
	if ok {
		w.jumpToSearchRow(row)
	}
	return ok
}

func (w *TableWidget) SearchPrev() bool {
	row, ok := tableSearchJump(w.table, w.selectedRow, -1, w.searchPattern)
	if ok {
		w.jumpToSearchRow(row)
	}
	return ok
}

func (w *TableWidget) jumpToSearchRow(row int) {
	w.SetSelectedRow(row)
	w.EnsureRowVisible()
	if w.onSearchJump != nil {
		w.onSearchJump(row)
	}
}

func (w *TableWidget) WordAtCursor() string {
	if w.selectedRow < 0 || w.selectedRow >= w.table.NumRows() {
		return ""
	}
	text := w.table.RowDisplayLine(w.selectedRow)
	return identAtOrNear(text, 0)
}

func (w *TableWidget) CursorInSearchMatch() bool {
	if w.searchPattern == "" {
		return false
	}
	return tableRowMatches(w.table, w.selectedRow, w.searchPattern)
}

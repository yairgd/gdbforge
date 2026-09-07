package termui

import (
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// TableSearchHit is a rune range within one cell to highlight for /search.
type TableSearchHit struct {
	Row, Col, Start, End int
}

// TablePaintState controls row styling and search highlights during paint.
type TablePaintState struct {
	RowStyle      func(row int) tcell.Style
	SearchColor   tcell.Color
	SearchHits    []TableSearchHit
	SelectHit     *TableSearchHit
	SelectColor   tcell.Color
}

func (ps TablePaintState) cellStyle(row, col, runeOff int, base tcell.Style) tcell.Style {
	st := base
	if ps.SelectHit != nil && ps.SelectHit.Row == row && ps.SelectHit.Col == col &&
		runeOff >= ps.SelectHit.Start && runeOff < ps.SelectHit.End {
		if ps.SelectColor != tcell.ColorDefault {
			st = st.Background(ps.SelectColor).Foreground(platform.ContrastColor(ps.SelectColor))
		}
	}
	for _, h := range ps.SearchHits {
		if h.Row == row && h.Col == col && runeOff >= h.Start && runeOff < h.End {
			if ps.SearchColor != tcell.ColorDefault {
				st = st.Background(ps.SearchColor).Foreground(platform.ContrastColor(ps.SearchColor))
			}
			break
		}
	}
	return st
}

func (ps TablePaintState) rowStyle(row int, defaultRow tcell.Style) tcell.Style {
	if ps.RowStyle != nil {
		return ps.RowStyle(row)
	}
	return defaultRow
}

func findCellSearchHits(row, col int, text, pattern string) [][2]int {
	if pattern == "" || text == "" {
		return nil
	}
	var spans [][2]int
	rs := []rune(text)
	low := strings.ToLower(string(rs))
	pat := strings.ToLower(pattern)
	off := 0
	for {
		i := strings.Index(low[off:], pat)
		if i < 0 {
			break
		}
		start := off + i
		end := start + utf8.RuneCountInString(pat)
		if end > len(rs) {
			end = len(rs)
		}
		spans = append(spans, [2]int{start, end})
		off = start + 1
	}
	_ = row
	_ = col
	return spans
}

// RebuildTableSearch scans table cells for pattern and returns highlight spans.
func RebuildTableSearch(t *Table, pattern string) []TableSearchHit {
	if t == nil || pattern == "" {
		return nil
	}
	var hits []TableSearchHit
	for row := 0; row < t.NumRows(); row++ {
		for col := 0; col < t.NumCols(); col++ {
			text := ""
			if col < len(t.rows[row]) {
				text = t.rows[row][col].Text
			}
			for _, span := range findCellSearchHits(row, col, text, pattern) {
				hits = append(hits, TableSearchHit{
					Row: row, Col: col, Start: span[0], End: span[1],
				})
			}
		}
	}
	return hits
}

func tableRowMatches(t *Table, row int, pattern string) bool {
	if pattern == "" || row < 0 || row >= t.NumRows() {
		return false
	}
	pat := strings.ToLower(pattern)
	for col := 0; col < t.NumCols(); col++ {
		text := ""
		if col < len(t.rows[row]) {
			text = t.rows[row][col].Text
		}
		if strings.Contains(strings.ToLower(text), pat) {
			return true
		}
	}
	return false
}

func tableSearchJump(t *Table, startRow, dir int, pattern string) (int, bool) {
	n := t.NumRows()
	if n == 0 || pattern == "" {
		return startRow, false
	}
	if startRow < 0 || startRow >= n {
		startRow = 0
	}
	for i := 0; i < n; i++ {
		row := startRow + dir*(i+1)
		for row < 0 {
			row += n
		}
		row %= n
		if tableRowMatches(t, row, pattern) {
			return row, true
		}
	}
	return startRow, false
}

// RowText returns tab-separated column texts for copy/search helpers.
func (t *Table) RowText(row int) string {
	if row < 0 || row >= len(t.rows) {
		return ""
	}
	parts := make([]string, t.NumCols())
	for col := 0; col < t.NumCols(); col++ {
		if col < len(t.rows[row]) {
			parts[col] = t.rows[row][col].Text
		}
	}
	return strings.Join(parts, "\t")
}

// RowDisplayLine joins columns with two spaces (legacy list pane format).
func (t *Table) RowDisplayLine(row int) string {
	if row < 0 || row >= len(t.rows) {
		return ""
	}
	parts := make([]string, 0, t.NumCols())
	for col := 0; col < t.NumCols(); col++ {
		if col < len(t.rows[row]) {
			parts = append(parts, t.rows[row][col].Text)
		} else {
			parts = append(parts, "")
		}
	}
	return strings.Join(parts, "  ")
}

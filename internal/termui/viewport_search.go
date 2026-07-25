package termui

import (
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// SearchHost is implemented by panes that support /search (Viewport-backed).
type SearchHost interface {
	SetSearchPattern(pattern string)
	CommitSearch(pattern string)
	RevertSearch()
	SearchNext() bool
	SearchPrev() bool
	SearchPattern() string
	SetSearchColor(c tcell.Color)
}

// SetSearchContentOffset skips this many visible columns when matching/highlighting
// (e.g. CodeWidget gutter width).
func (v *Viewport) SetSearchContentOffset(n int) {
	if v == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	v.searchContentOffset = n
}

// SetSearchColor sets the background used for matching substrings.
func (v *Viewport) SetSearchColor(c tcell.Color) {
	if v == nil {
		return
	}
	v.searchColor = c
}

// SetOnSearchJump registers a callback after SearchNext/Prev/Commit moves the line
// (list panes sync their selected row here).
func (v *Viewport) SetOnSearchJump(fn func(lineIdx int)) {
	if v == nil {
		return
	}
	v.onSearchJump = fn
}

// SetSearchPattern updates the live /search highlight (does not commit).
func (v *Viewport) SetSearchPattern(pattern string) {
	if v == nil {
		return
	}
	v.searchPattern = pattern
}

// CommitSearch stores pattern as the lasting highlight and jumps to a match.
func (v *Viewport) CommitSearch(pattern string) {
	if v == nil {
		return
	}
	v.searchPattern = pattern
	v.searchCommitted = pattern
	if pattern == "" {
		return
	}
	if v.lineMatches(v.CursorLine) {
		v.jumpToSearchLine(v.CursorLine)
		return
	}
	_ = v.SearchNext()
}

// RevertSearch restores the last committed pattern (Esc from /search).
func (v *Viewport) RevertSearch() {
	if v == nil {
		return
	}
	v.searchPattern = v.searchCommitted
}

// SearchPattern returns the live search text.
func (v *Viewport) SearchPattern() string {
	if v == nil {
		return ""
	}
	return v.searchPattern
}

// SearchNext moves to the next matching line (wraps).
func (v *Viewport) SearchNext() bool {
	return v.searchJump(1)
}

// SearchPrev moves to the previous matching line (wraps).
func (v *Viewport) SearchPrev() bool {
	return v.searchJump(-1)
}

func (v *Viewport) searchJump(dir int) bool {
	if v == nil || v.searchPattern == "" || v.Buffer == nil {
		return false
	}
	n := v.lineLimit()
	if n <= 0 {
		return false
	}
	start := v.CursorLine
	if start < 0 {
		start = 0
	}
	if start >= n {
		start = n - 1
	}
	for step := 1; step <= n; step++ {
		idx := (start + dir*step) % n
		if idx < 0 {
			idx += n
		}
		if v.lineMatches(idx) {
			v.jumpToSearchLine(idx)
			return true
		}
	}
	return false
}

func (v *Viewport) jumpToSearchLine(lineIdx int) {
	v.CursorLine = lineIdx
	v.CursorCol = 0
	v.EnsureCursorVisible()
	if v.onSearchJump != nil {
		v.onSearchJump(lineIdx)
	}
}

func (v *Viewport) lineMatches(lineIdx int) bool {
	if v.searchPattern == "" || v.Buffer == nil {
		return false
	}
	return strings.Contains(v.searchContent(lineIdx), v.searchPattern)
}

// searchContent returns printable line text with the gutter offset removed.
func (v *Viewport) searchContent(lineIdx int) string {
	if v.Buffer == nil || lineIdx < 0 || lineIdx >= v.Buffer.NumLines() {
		return ""
	}
	plain := []rune(platform.StripANSI(v.Buffer.Line(lineIdx)))
	off := v.searchContentOffset
	if off > len(plain) {
		return ""
	}
	return string(plain[off:])
}

func (v *Viewport) searchBG() tcell.Color {
	if v.searchColor == tcell.ColorDefault {
		return platform.DefaultSearchColor
	}
	return v.searchColor
}

// applySearchStyle paints matching substrings (content columns only).
func (v *Viewport) applySearchStyle(lineIdx, absVisCol int, st tcell.Style) tcell.Style {
	if v.searchPattern == "" {
		return st
	}
	contentCol := absVisCol - v.searchContentOffset
	if contentCol < 0 {
		return st
	}
	if v.runeInSearchMatch(lineIdx, contentCol) {
		bg := v.searchBG()
		return st.Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (v *Viewport) runeInSearchMatch(lineIdx, contentCol int) bool {
	if v.searchPattern == "" || contentCol < 0 {
		return false
	}
	line := v.searchContent(lineIdx)
	pat := v.searchPattern
	if pat == "" || line == "" {
		return false
	}
	for start := 0; start <= len(line); {
		rel := strings.Index(line[start:], pat)
		if rel < 0 {
			return false
		}
		byteStart := start + rel
		runeStart := utf8.RuneCountInString(line[:byteStart])
		runeEnd := runeStart + utf8.RuneCountInString(pat)
		if contentCol >= runeStart && contentCol < runeEnd {
			return true
		}
		next := byteStart + len(pat)
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return false
}

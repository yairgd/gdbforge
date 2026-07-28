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

// WordAtCursor returns the identifier under the cursor on searchable content
// (letters/digits/_). Punctuation runs like "===" are skipped so */# on a
// banner line such as "=== sdk_cpp_demo done ===" searches sdk_cpp_demo.
func (v *Viewport) WordAtCursor() string {
	if v == nil || v.Buffer == nil {
		return ""
	}
	content := v.searchContent(v.CursorLine)
	if content == "" {
		return ""
	}
	return identAtOrNear(content, v.contentByteAtCursor())
}

// CursorInSearchMatch reports whether the caret sits inside a highlighted
// match of the live search pattern (e.g. after /46 on "1052946").
func (v *Viewport) CursorInSearchMatch() bool {
	if v == nil || v.searchPattern == "" || v.Buffer == nil {
		return false
	}
	content := v.searchContent(v.CursorLine)
	if content == "" {
		return false
	}
	byteAt := v.contentByteAtCursor()
	if byteAt < 0 {
		byteAt = 0
	}
	if byteAt > len(content) {
		byteAt = len(content)
	}
	contentCol := utf8.RuneCountInString(content[:byteAt])
	// At EOL, treat as still on the last rune so */# after /pattern do not
	// expand to the enclosing identifier.
	if contentCol > 0 && byteAt >= len(content) {
		contentCol--
	}
	return v.runeInSearchMatch(v.CursorLine, contentCol)
}

// identAtOrNear returns the isWordChar token at/near byte offset at.
// Prefer the token under at; if that is punctuation/empty, the next identifier
// forward, then the previous, then the first on the line.
func identAtOrNear(line string, at int) string {
	if line == "" {
		return ""
	}
	if at < 0 {
		at = 0
	}
	if at > len(line) {
		at = len(line)
	}
	if w := identBoundsAt(line, at); w != "" {
		return w
	}
	// Walk forward from at for an identifier start.
	for i := at; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if isWordChar(r) {
			return identBoundsAt(line, i)
		}
		i += size
	}
	// Walk backward.
	for i := at; i > 0; {
		r, size := utf8.DecodeLastRuneInString(line[:i])
		i -= size
		if isWordChar(r) {
			return identBoundsAt(line, i)
		}
	}
	// First identifier on the line.
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if isWordChar(r) {
			return identBoundsAt(line, i)
		}
		i += size
	}
	return ""
}

// identBoundsAt returns the isWordChar span covering at, or "" if at is not
// on an identifier.
func identBoundsAt(line string, at int) string {
	if at < 0 {
		at = 0
	}
	if at >= len(line) {
		if at == 0 {
			return ""
		}
		at = len(line) - 1
		// Snap back to rune start.
		for at > 0 && !utf8.RuneStart(line[at]) {
			at--
		}
	}
	r, _ := utf8.DecodeRuneInString(line[at:])
	if !isWordChar(r) {
		return ""
	}
	s, e := wordBoundsAt(line, at)
	if s >= e || s < 0 || e > len(line) {
		return ""
	}
	// wordBoundsAt may return a punctuation token; require identifier class.
	rr, _ := utf8.DecodeRuneInString(line[s:])
	if !isWordChar(rr) {
		return ""
	}
	return line[s:e]
}

// contentByteAtCursor maps CursorCol onto a byte offset in searchContent.
func (v *Viewport) contentByteAtCursor() int {
	content := v.searchContent(v.CursorLine)
	if content == "" {
		return 0
	}
	visCol := 0
	if v.CursorLine >= 0 && v.CursorLine < v.Buffer.NumLines() {
		raw := v.Buffer.Line(v.CursorLine)
		switch {
		case v.CursorCol <= 0:
			visCol = 0
		case v.ANSI:
			if v.CursorCol >= len(raw) {
				visCol = VisibleANSIWidth(raw)
			} else {
				visCol = VisibleANSIWidth(raw[:v.CursorCol])
			}
		default:
			visCol = utf8.RuneCountInString(raw[:min(v.CursorCol, len(raw))])
		}
	}
	col := visCol - v.searchContentOffset
	if col < 0 {
		col = 0
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return 0
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}
	return len(string(runes[:col]))
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
	// Consoles start in follow-tail; leaving it keeps Draw from ScrollToBottom
	// and undoing the match (IO / GDB / Exec panes).
	v.leaveFollowTail()
	v.CursorLine = lineIdx
	v.placeCursorOnSearchMatch(lineIdx)
	v.EnsureCursorVisible()
	if v.onSearchJump != nil {
		v.onSearchJump(lineIdx)
	}
}

// placeCursorOnSearchMatch puts CursorCol on the first pattern match in the line.
func (v *Viewport) placeCursorOnSearchMatch(lineIdx int) {
	if v.Buffer == nil || v.searchPattern == "" {
		v.CursorCol = 0
		return
	}
	content := v.searchContent(lineIdx)
	rel := strings.Index(content, v.searchPattern)
	contentCol := 0
	if rel >= 0 {
		contentCol = utf8.RuneCountInString(content[:rel])
	}
	vis := v.searchContentOffset + contentCol
	raw := v.Buffer.Line(lineIdx)
	if v.ANSI {
		v.CursorCol = ANSIByteIndexAtVisible(raw, vis)
		return
	}
	runes := []rune(platform.StripANSI(raw))
	if vis > len(runes) {
		vis = len(runes)
	}
	v.CursorCol = len(string(runes[:vis]))
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

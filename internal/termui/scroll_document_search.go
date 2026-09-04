package termui

import (
	"strings"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// SearchHost is implemented by panes that support /search (ScrollDocument-backed).
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
func (d *ScrollDocument) SetSearchContentOffset(n int) {
	if d == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	d.searchContentOffset = n
}

// SetSearchColor sets the background used for matching substrings.
func (d *ScrollDocument) SetSearchColor(c tcell.Color) {
	if d == nil {
		return
	}
	d.searchColor = c
}

// SetOnSearchJump registers a callback after SearchNext/Prev/Commit moves the line
// (list panes sync their selected row here).
func (d *ScrollDocument) SetOnSearchJump(fn func(lineIdx int)) {
	if d == nil {
		return
	}
	d.onSearchJump = fn
}

// SetSearchPattern updates the live /search highlight (does not commit).
func (d *ScrollDocument) SetSearchPattern(pattern string) {
	if d == nil {
		return
	}
	d.searchPattern = pattern
}

// CommitSearch stores pattern as the lasting highlight and jumps to a match.
func (d *ScrollDocument) CommitSearch(pattern string) {
	if d == nil {
		return
	}
	d.searchPattern = pattern
	d.searchCommitted = pattern
	if pattern == "" {
		return
	}
	if d.lineMatches(d.CursorLine) {
		d.jumpToSearchLine(d.CursorLine)
		return
	}
	_ = d.SearchNext()
}

// RevertSearch restores the last committed pattern (Esc from /search).
func (d *ScrollDocument) RevertSearch() {
	if d == nil {
		return
	}
	d.searchPattern = d.searchCommitted
}

// SearchPattern returns the live search text.
func (d *ScrollDocument) SearchPattern() string {
	if d == nil {
		return ""
	}
	return d.searchPattern
}

// SearchNext moves to the next matching line (wraps).
func (d *ScrollDocument) SearchNext() bool {
	return d.searchJump(1)
}

// SearchPrev moves to the previous matching line (wraps).
func (d *ScrollDocument) SearchPrev() bool {
	return d.searchJump(-1)
}

// WordAtCursor returns the identifier under the cursor on searchable content
// (letters/digits/_). Punctuation runs like "===" are skipped so */# on a
// banner line such as "=== sdk_cpp_demo done ===" searches sdk_cpp_demo.
func (d *ScrollDocument) WordAtCursor() string {
	if d == nil || d.Buffer == nil {
		return ""
	}
	content := d.searchContent(d.CursorLine)
	if content == "" {
		return ""
	}
	return identAtOrNear(content, d.contentByteAtCursor())
}

// CursorInSearchMatch reports whether the caret sits inside a highlighted
// match of the live search pattern (e.g. after /46 on "1052946").
func (d *ScrollDocument) CursorInSearchMatch() bool {
	if d == nil || d.searchPattern == "" || d.Buffer == nil {
		return false
	}
	content := d.searchContent(d.CursorLine)
	if content == "" {
		return false
	}
	byteAt := d.contentByteAtCursor()
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
	return d.runeInSearchMatch(d.CursorLine, contentCol)
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
func (d *ScrollDocument) contentByteAtCursor() int {
	content := d.searchContent(d.CursorLine)
	if content == "" {
		return 0
	}
	visCol := 0
	if d.CursorLine >= 0 && d.CursorLine < d.Buffer.NumLines() {
		raw := d.Buffer.Line(d.CursorLine)
		switch {
		case d.CursorCol <= 0:
			visCol = 0
		default:
			visCol = utf8.RuneCountInString(raw[:min(d.CursorCol, len(raw))])
		}
	}
	col := visCol - d.searchContentOffset
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

func (d *ScrollDocument) searchJump(dir int) bool {
	if d == nil || d.searchPattern == "" || d.Buffer == nil {
		return false
	}
	n := d.lineLimit()
	if n <= 0 {
		return false
	}
	start := d.CursorLine
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
		if d.lineMatches(idx) {
			d.jumpToSearchLine(idx)
			return true
		}
	}
	return false
}

func (d *ScrollDocument) jumpToSearchLine(lineIdx int) {
	// Consoles start in follow-tail; leaving it keeps Draw from ScrollToBottom
	// and undoing the match (IO / GDB / Exec panes).
	d.leaveFollowTail()
	d.CursorLine = lineIdx
	d.placeCursorOnSearchMatch(lineIdx)
	d.EnsureCursorVisible()
	if d.onSearchJump != nil {
		d.onSearchJump(lineIdx)
	}
}

// placeCursorOnSearchMatch puts CursorCol on the first pattern match in the line.
func (d *ScrollDocument) placeCursorOnSearchMatch(lineIdx int) {
	if d.Buffer == nil || d.searchPattern == "" {
		d.CursorCol = 0
		return
	}
	content := d.searchContent(lineIdx)
	rel := strings.Index(content, d.searchPattern)
	contentCol := 0
	if rel >= 0 {
		contentCol = utf8.RuneCountInString(content[:rel])
	}
	vis := d.searchContentOffset + contentCol
	raw := d.Buffer.Line(lineIdx)
	runes := []rune(raw)
	if vis > len(runes) {
		vis = len(runes)
	}
	d.CursorCol = len(string(runes[:vis]))
}

func (d *ScrollDocument) lineMatches(lineIdx int) bool {
	if d.searchPattern == "" || d.Buffer == nil {
		return false
	}
	return strings.Contains(d.searchContent(lineIdx), d.searchPattern)
}

// searchContent returns printable line text with the gutter offset removed.
func (d *ScrollDocument) searchContent(lineIdx int) string {
	if d.Buffer == nil || lineIdx < 0 || lineIdx >= d.Buffer.NumLines() {
		return ""
	}
	line := d.Buffer.Line(lineIdx)
	plain := []rune(line)
	off := d.searchContentOffset
	if off > len(plain) {
		return ""
	}
	return string(plain[off:])
}

func (d *ScrollDocument) searchBG() tcell.Color {
	if d.searchColor == tcell.ColorDefault {
		return platform.DefaultSearchColor
	}
	return d.searchColor
}

// applySearchStyle paints matching substrings (content columns only).
func (d *ScrollDocument) applySearchStyle(lineIdx, absVisCol int, st tcell.Style) tcell.Style {
	if d.searchPattern == "" {
		return st
	}
	contentCol := absVisCol - d.searchContentOffset
	if contentCol < 0 {
		return st
	}
	if d.runeInSearchMatch(lineIdx, contentCol) {
		bg := d.searchBG()
		return st.Background(bg).Foreground(platform.ContrastColor(bg))
	}
	return st
}

func (d *ScrollDocument) runeInSearchMatch(lineIdx, contentCol int) bool {
	if d.searchPattern == "" || contentCol < 0 {
		return false
	}
	line := d.searchContent(lineIdx)
	pat := d.searchPattern
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

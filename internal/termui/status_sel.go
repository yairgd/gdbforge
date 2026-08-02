package termui

import (
	"time"
	"unicode"
	"unicode/utf8"
)

// statusLabelPrefixCols is the screen width of "▎ " before the focused name.
const statusLabelPrefixCols = 2

// statusSel holds mouse selection on a leaf's status band (outside content).
type statusSel struct {
	leaf         *Node
	label        string // name without "▎ "
	nameStartCol int    // local x where name starts (prefix or inactiveNameCol)
	anchor       int    // rune index into label
	cursor       int    // rune index into label
	dragging     bool
	hasSel       bool
	suppressDrag bool // after double/triple-click until release
	clickCount   int
	lastClickAt  time.Time
	lastClickCol int
}

func (s *statusSel) clear() {
	*s = statusSel{}
}

// clearHighlight drops the white selection but keeps multi-click timing.
func (s *statusSel) clearHighlight() {
	if s == nil {
		return
	}
	s.hasSel = false
	s.dragging = false
	s.suppressDrag = false
	s.anchor = 0
	s.cursor = 0
	s.label = ""
}

// statusLabelEndCol is the first local column after "▎ "+label (exclusive).
func statusLabelEndCol(label string) int {
	return statusLabelPrefixCols + utf8.RuneCountInString(label)
}

func (s *statusSel) ordered() (start, end int) {
	start, end = s.anchor, s.cursor
	if start > end {
		start, end = end, start
	}
	return start, end
}

func (s *statusSel) selectedText() string {
	if s == nil || s.label == "" {
		return ""
	}
	runes := []rune(s.label)
	start, end := s.ordered()
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// StatusLabeler is implemented by panes that expose copyable status-band text.
type StatusLabeler interface {
	StatusLabel() string
}

// statusLabelOf returns the pane status text (full path for Code, etc.).
func statusLabelOf(w Widget) string {
	if w == nil {
		return ""
	}
	if s, ok := w.(StatusLabeler); ok {
		return s.StatusLabel()
	}
	return ""
}

// isStatusWordChar: path-friendly word (main.c as one token; / breaks).
func isStatusWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-'
}

// statusWordBounds returns [start,end) rune indices of the word under at.
func statusWordBounds(label string, at int) (start, end int) {
	runes := []rune(label)
	n := len(runes)
	if n == 0 {
		return 0, 0
	}
	if at < 0 {
		at = 0
	}
	if at >= n {
		at = n - 1
	}
	if unicode.IsSpace(runes[at]) {
		return at, at
	}
	same := isStatusWordChar(runes[at])
	start = at
	for start > 0 {
		prev := runes[start-1]
		if unicode.IsSpace(prev) || isStatusWordChar(prev) != same {
			break
		}
		start--
	}
	end = at + 1
	for end < n {
		next := runes[end]
		if unicode.IsSpace(next) || isStatusWordChar(next) != same {
			break
		}
		end++
	}
	return start, end
}

func statusBandContains(r Rect, x, y int) bool {
	return y == r.Bottom() && x >= r.X() && x < r.Right()
}

func runeIndexAtLocalX(label string, localX, nameStartCol int) int {
	col := localX - nameStartCol
	if col < 0 {
		return 0
	}
	n := utf8.RuneCountInString(label)
	if col >= n {
		return n
	}
	return col
}

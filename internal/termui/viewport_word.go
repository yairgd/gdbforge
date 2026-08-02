package termui

import (
	"unicode"
	"unicode/utf8"

	tcell "github.com/gdamore/tcell/v2"
)

const (
	clickMultiTimeoutMs = 400
)

// isWordChar reports characters that form a "word" under double-click
// (letters, digits, underscore — same class most terminals use).
func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordBoundsAt returns [start,end) byte offsets of the word (or non-space
// token) under at in line. ANSI/OSC escapes are skipped and never selected.
func wordBoundsAt(line string, at int) (start, end int) {
	if line == "" {
		return 0, 0
	}
	if at < 0 {
		at = 0
	}
	if at > len(line) {
		at = len(line)
	}
	at = snapToContent(line, at)
	if at >= len(line) {
		return len(line), len(line)
	}

	r, next, ok := nextContentRune(line, at)
	if !ok {
		return at, at
	}
	if unicode.IsSpace(r) {
		return at, at
	}

	sameClass := isWordChar(r)
	start = at
	for {
		prev, ok := prevContentRune(line, start)
		if !ok {
			break
		}
		if unicode.IsSpace(prev.r) || isWordChar(prev.r) != sameClass {
			break
		}
		start = prev.at
	}

	end = next
	for end < len(line) {
		nr, nnext, ok := nextContentRune(line, end)
		if !ok {
			break
		}
		if unicode.IsSpace(nr) || isWordChar(nr) != sameClass {
			break
		}
		end = nnext
	}
	return start, end
}

type contentRune struct {
	at int
	r  rune
}

// snapToContent moves at forward past any escape so it lands on a printable
// rune (or EOF). Unknown/incomplete ESC bytes are skipped one at a time.
func snapToContent(line string, at int) int {
	style := tcell.StyleDefault
	for at < len(line) {
		if line[at] != 0x1b {
			return at
		}
		next, _, ok := consumeEscape(line, at, style, style)
		if ok {
			at = next
			continue
		}
		at++
	}
	return at
}

// nextContentRune returns the next printable rune at/after at, and the byte
// index immediately after that rune (escapes between at and the rune are
// included in the skip via snapToContent).
func nextContentRune(line string, at int) (r rune, next int, ok bool) {
	at = snapToContent(line, at)
	if at >= len(line) {
		return 0, at, false
	}
	r, sz := utf8.DecodeRuneInString(line[at:])
	if sz <= 0 {
		return 0, at, false
	}
	return r, at + sz, true
}

func prevContentRune(line string, at int) (contentRune, bool) {
	style := tcell.StyleDefault
	i := at
	for i > 0 {
		_, sz := utf8.DecodeLastRuneInString(line[:i])
		if sz <= 0 {
			return contentRune{}, false
		}
		i -= sz
		if line[i] == 0x1b {
			next, _, ok := consumeEscape(line, i, style, style)
			if ok && next <= at {
				continue
			}
		}
		if isInsideANSI(line, i) {
			continue
		}
		r, _ := utf8.DecodeRuneInString(line[i:])
		return contentRune{at: i, r: r}, true
	}
	return contentRune{}, false
}

func isInsideANSI(line string, at int) bool {
	style := tcell.StyleDefault
	for i := 0; i <= at && i < len(line); {
		if line[i] != 0x1b {
			_, sz := utf8.DecodeRuneInString(line[i:])
			if sz <= 0 {
				return false
			}
			i += sz
			continue
		}
		next, _, ok := consumeEscape(line, i, style, style)
		if !ok {
			return false
		}
		if at >= i && at < next {
			return true
		}
		i = next
	}
	return false
}

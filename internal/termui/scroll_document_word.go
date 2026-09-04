package termui

import (
	"unicode"
	"unicode/utf8"
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
// token) under at in line.
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
	at = snapToRuneBoundary(line, at)
	if at >= len(line) {
		return len(line), len(line)
	}

	r, next, ok := nextRune(line, at)
	if !ok {
		return at, at
	}
	if unicode.IsSpace(r) {
		return at, at
	}

	sameClass := isWordChar(r)
	start = at
	for {
		prev, ok := prevRune(line, start)
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
		nr, nnext, ok := nextRune(line, end)
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

func snapToRuneBoundary(line string, at int) int {
	if at <= 0 || at >= len(line) {
		return at
	}
	for at > 0 && !utf8.RuneStart(line[at]) {
		at--
	}
	return at
}

func nextRune(line string, at int) (r rune, next int, ok bool) {
	at = snapToRuneBoundary(line, at)
	if at >= len(line) {
		return 0, at, false
	}
	r, sz := utf8.DecodeRuneInString(line[at:])
	if sz <= 0 {
		return 0, at, false
	}
	return r, at + sz, true
}

func prevRune(line string, at int) (contentRune, bool) {
	i := at
	for i > 0 {
		_, sz := utf8.DecodeLastRuneInString(line[:i])
		if sz <= 0 {
			return contentRune{}, false
		}
		i -= sz
		r, _ := utf8.DecodeRuneInString(line[i:])
		return contentRune{at: i, r: r}, true
	}
	return contentRune{}, false
}

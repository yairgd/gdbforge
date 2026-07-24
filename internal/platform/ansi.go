package platform

import (
	"strings"
	"unicode/utf8"
)

// StripANSI removes OSC/CSI/SGR escape sequences, leaving printable text.
// Framework helper so backends (e.g. dlv) need not import termui.
func StripANSI(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		if text[i] == 0x1b {
			if next, ok := skipEscape(text, i); ok {
				i = next
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// skipEscape advances past a CSI/OSC/other ESC sequence starting at i.
func skipEscape(text string, i int) (next int, ok bool) {
	if i >= len(text) || text[i] != 0x1b {
		return i, false
	}
	i++
	if i >= len(text) {
		return i - 1, false
	}
	switch text[i] {
	case '[': // CSI: ESC [ ... final byte @-~
		i++
		for i < len(text) {
			c := text[i]
			i++
			if c >= 0x40 && c <= 0x7e {
				return i, true
			}
		}
		return i, true
	case ']': // OSC: ESC ] ... BEL or ST (ESC \)
		i++
		for i < len(text) {
			if text[i] == 0x07 {
				return i + 1, true
			}
			if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '\\' {
				return i + 2, true
			}
			i++
		}
		return i, true
	default:
		// Two-byte ESC sequences (e.g. ESC c) — skip ESC + one byte.
		return i + 1, true
	}
}

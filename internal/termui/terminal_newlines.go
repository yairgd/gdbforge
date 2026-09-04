package termui

import "strings"

// TerminalNewlines converts plain \n (and normalizes \r\n / \r) to \r\n pairs
// for WriteRaw into an xterm buffer. LF-only text otherwise advances the cursor
// vertically without returning to column 0.
func TerminalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(s, "\n", "\r\n")
}

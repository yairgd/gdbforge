package termui

import (
	"strings"
	"unicode/utf8"

	xterm "github.com/gitpod-io/xterm-go"
)

// debuggerPrompts are stripped from the start of the xterm input line.
var debuggerPrompts = []string{
	"(gdb) ",
	"(dlv) ",
	"> ",
}

// InputLineText returns the editable portion of the current xterm line up to the
// cursor (debugger prompt stripped). Used for GDB/Delve Tab completion.
func InputLineText(c *TerminalController) string {
	if c == nil {
		return ""
	}
	var line string
	c.WithTerminal(func(term *xterm.Terminal) {
		cx, cy := term.CursorX(), term.CursorY()
		line = readLineRunes(term, cx, cy)
	})
	return stripDebuggerPrompt(strings.TrimRight(line, " \t"))
}

// ReplaceInputLine replaces the editable input on the current line with newText
// by sending backspaces and new keystrokes to the attached PTY/readline.
func ReplaceInputLine(c *TerminalController, newText string) {
	if c == nil {
		return
	}
	cur := InputLineText(c)
	if newText == cur {
		return
	}
	n := utf8.RuneCountInString(cur)
	for i := 0; i < n; i++ {
		_ = c.SendInput([]byte("\x7f"))
	}
	if newText != "" {
		_ = c.SendString(newText)
	}
}

func readLineRunes(term *xterm.Terminal, cx, cy int) string {
	if term == nil || cx < 0 || cy < 0 {
		return ""
	}
	buf := term.Buffer()
	line := buf.Lines.Get(buf.YDisp + cy)
	if line == nil {
		return ""
	}
	if cx >= term.Cols() {
		cx = term.Cols() - 1
	}
	var b strings.Builder
	cell := xterm.NewCellData()
	for x := 0; x <= cx; x++ {
		line.LoadCell(x, cell)
		ch := ' '
		if chars := cell.GetChars(); chars != "" {
			for _, r := range chars {
				ch = r
				break
			}
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func stripDebuggerPrompt(line string) string {
	for _, p := range debuggerPrompts {
		if strings.HasPrefix(line, p) {
			return line[len(p):]
		}
	}
	if strings.HasPrefix(line, "(gdb)") {
		return strings.TrimSpace(line[len("(gdb)"):])
	}
	if strings.HasPrefix(line, "(dlv)") {
		return strings.TrimSpace(line[len("(dlv)"):])
	}
	return strings.TrimSpace(line)
}

package termui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	xterm "github.com/gitpod-io/xterm-go"
)

// debuggerPrompts are stripped from the start of the xterm input line.
var debuggerPrompts = []string{
	"(gdb) ",
	"(dlv) ",
	"> ",
	"lua> ",
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

// FullInputLineText returns the full editable portion of the current row
// (prompt stripped), not just the text before the cursor.
func FullInputLineText(c *TerminalController, prompt string) string {
	text, _ := PromptInputState(c, prompt)
	return text
}

// PromptInputState returns editable text on the current row and the cursor
// offset within that text (in bytes, matching xterm cell columns for BMP input).
func PromptInputState(c *TerminalController, prompt string) (text string, cursor int) {
	if c == nil {
		return "", 0
	}
	var cx, cy int
	var full string
	c.WithTerminal(func(term *xterm.Terminal) {
		cx, cy = term.CursorX(), term.CursorY()
		full = readFullLineRunes(term, cy)
	})
	full = strings.TrimRight(full, " \t")
	p := strings.TrimRight(prompt, " \t")
	switch {
	case strings.HasPrefix(full, prompt):
		text = peelLeadingPrompt(strings.TrimSpace(full[len(prompt):]), prompt)
		cursor = cx - len(prompt)
	case p != "" && strings.HasPrefix(full, p):
		text = peelLeadingPrompt(strings.TrimSpace(full[len(p):]), prompt)
		cursor = cx - len(p)
	default:
		text = stripDebuggerPrompt(full)
		cursor = cx
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(text) {
		cursor = len(text)
	}
	return text, cursor
}

// OnPromptLine reports whether the cursor row already starts with prompt.
func OnPromptLine(c *TerminalController, prompt string) bool {
	if c == nil || prompt == "" {
		return false
	}
	full := promptLineRaw(c)
	p := strings.TrimRight(prompt, " \t")
	if !strings.HasPrefix(full, p) {
		return false
	}
	rest := strings.TrimSpace(full[len(p):])
	if rest == "" {
		return true
	}
	// Reject duplicated prompts (e.g. "lua> lua>").
	return !strings.HasPrefix(rest, p)
}

// OnEmptyPromptLine reports whether the cursor is on prompt with no user input.
func OnEmptyPromptLine(c *TerminalController, prompt string) bool {
	if !OnPromptLine(c, prompt) {
		return false
	}
	text, _ := PromptInputState(c, prompt)
	return strings.TrimSpace(text) == ""
}

func promptLineRaw(c *TerminalController) string {
	if c == nil {
		return ""
	}
	var full string
	c.WithTerminal(func(term *xterm.Terminal) {
		_, cy := term.CursorX(), term.CursorY()
		full = readFullLineRunes(term, cy)
	})
	return strings.TrimRight(full, " \t")
}

// MovePromptCursor moves the caret within the editable portion after prompt.
func MovePromptCursor(c *TerminalController, prompt string, editableCol int) {
	if c == nil {
		return
	}
	if editableCol < 0 {
		editableCol = 0
	}
	_, cur := PromptInputState(c, prompt)
	delta := editableCol - cur
	if delta == 0 {
		return
	}
	if delta > 0 {
		_ = c.WriteString(fmt.Sprintf("\x1b[%dC", delta))
	} else {
		_ = c.WriteString(fmt.Sprintf("\x1b[%dD", -delta))
	}
}

// CurrentLine returns the xterm row contents from column 0 through the cursor.
func CurrentLine(c *TerminalController) string {
	return strings.TrimRight(currentLineRaw(c), " \t")
}

func currentLineRaw(c *TerminalController) string {
	if c == nil {
		return ""
	}
	var line string
	c.WithTerminal(func(term *xterm.Terminal) {
		cx, cy := term.CursorX(), term.CursorY()
		line = readLineRunes(term, cx, cy)
	})
	return line
}

// RewritePromptInput replaces the entire editable line after prompt on the
// current row. Use for local-echo REPLs (Lua); PTY-backed consoles should keep
// ReplaceInputLine so the remote readline owns the buffer.
func RewritePromptInput(c *TerminalController, prompt, newText string) {
	if c == nil {
		return
	}
	_ = c.WriteString("\r" + prompt + newText + "\x1b[K")
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

// ApplyCompletion inserts full into the PTY input line. When full extends cur,
// only the new suffix is sent — the PTY already echoed cur. Otherwise the
// editable line is replaced (wildmenu token swap, non-prefix completion).
func ApplyCompletion(c *TerminalController, cur, full string) {
	if c == nil || full == "" {
		return
	}
	if full == cur {
		return
	}
	if cur != "" && strings.HasPrefix(full, cur) {
		if suffix := full[len(cur):]; suffix != "" {
			_ = c.SendString(suffix)
		}
		return
	}
	ReplaceInputLine(c, full)
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

func readFullLineRunes(term *xterm.Terminal, cy int) string {
	if term == nil || cy < 0 {
		return ""
	}
	buf := term.Buffer()
	line := buf.Lines.Get(buf.YDisp + cy)
	if line == nil {
		return ""
	}
	var b strings.Builder
	cell := xterm.NewCellData()
	for x := 0; x < term.Cols(); x++ {
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

func peelLeadingPrompt(text, prompt string) string {
	s := strings.TrimSpace(text)
	p := strings.TrimRight(prompt, " \t")
	for {
		trimmed := false
		if strings.HasPrefix(s, prompt) {
			s = strings.TrimSpace(s[len(prompt):])
			trimmed = true
		} else if p != "" && strings.HasPrefix(s, p) {
			s = strings.TrimSpace(s[len(p):])
			trimmed = true
		}
		if !trimmed {
			break
		}
	}
	return s
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

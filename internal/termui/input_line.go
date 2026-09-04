package termui

import (
	tcell "github.com/gdamore/tcell/v2"
)

// KeyBinder is satisfied by BaseWidget for registering readline chords.
type KeyBinder interface {
	BindKeyFunc(name string, action func(args ...any), bindings ...string)
}

// InputLine is a reusable single-line editor with readline-style history.
// It does not own layout or scrollback — CmdWidget (or a terminal pane) does.
type InputLine struct {
	text      string
	cursor    int
	history   []string
	histIndex int // len(history) means editing a new/draft line
	histDraft string
}

func NewInputLine() *InputLine {
	return &InputLine{}
}

func (l *InputLine) Text() string   { return l.text }
func (l *InputLine) CursorCol() int { return l.cursor }

func (l *InputLine) SetText(s string) {
	l.text = s
	l.cursor = len(s)
}

func (l *InputLine) Clear() {
	l.text = ""
	l.cursor = 0
	l.histDraft = ""
	l.histIndex = len(l.history)
}

// LastHistory returns the most recently pushed non-empty command, or "".
func (l *InputLine) LastHistory() string {
	if n := len(l.history); n > 0 {
		return l.history[n-1]
	}
	return ""
}

func (l *InputLine) InsertRune(r rune) {
	s := string(r)
	l.text = l.text[:l.cursor] + s + l.text[l.cursor:]
	l.cursor += len(s)
}

// InsertText inserts s at the caret (used for clipboard paste).
func (l *InputLine) InsertText(s string) {
	if s == "" {
		return
	}
	l.text = l.text[:l.cursor] + s + l.text[l.cursor:]
	l.cursor += len(s)
}

func (l *InputLine) MoveLeft() {
	if l.cursor > 0 {
		l.cursor--
	}
}

func (l *InputLine) MoveRight() {
	if l.cursor < len(l.text) {
		l.cursor++
	}
}

func (l *InputLine) MoveHome() { l.cursor = 0 }
func (l *InputLine) MoveEnd()  { l.cursor = len(l.text) }

func (l *InputLine) Backspace() {
	if l.cursor > 0 {
		l.text = l.text[:l.cursor-1] + l.text[l.cursor:]
		l.cursor--
	}
}

func (l *InputLine) DeleteForward() {
	if l.cursor < len(l.text) {
		l.text = l.text[:l.cursor] + l.text[l.cursor+1:]
	}
}

func (l *InputLine) KillToEnd() {
	if l.cursor < len(l.text) {
		l.text = l.text[:l.cursor]
	}
}

func (l *InputLine) KillToStart() {
	if l.cursor > 0 {
		l.text = l.text[l.cursor:]
		l.cursor = 0
	}
}

func (l *InputLine) KillWord() {
	if l.cursor == 0 {
		return
	}
	i := l.cursor
	for i > 0 && l.text[i-1] == ' ' {
		i--
	}
	for i > 0 && l.text[i-1] != ' ' {
		i--
	}
	l.text = l.text[:i] + l.text[l.cursor:]
	l.cursor = i
}

func (l *InputLine) HistoryPrev() {
	if len(l.history) == 0 {
		return
	}
	if l.histIndex == len(l.history) {
		l.histDraft = l.text
	}
	if l.histIndex > 0 {
		l.histIndex--
		l.text = l.history[l.histIndex]
		l.cursor = len(l.text)
	}
}

func (l *InputLine) HistoryNext() {
	if l.histIndex >= len(l.history) {
		return
	}
	l.histIndex++
	if l.histIndex == len(l.history) {
		l.text = l.histDraft
	} else {
		l.text = l.history[l.histIndex]
	}
	l.cursor = len(l.text)
}

// PushHistory records cmd (skips empty and consecutive duplicates).
func (l *InputLine) PushHistory(cmd string) {
	if cmd == "" {
		return
	}
	if n := len(l.history); n > 0 && l.history[n-1] == cmd {
		l.histIndex = n
		l.histDraft = ""
		return
	}
	l.history = append(l.history, cmd)
	l.histIndex = len(l.history)
	l.histDraft = ""
}

// BindKeys registers readline editing/history chords on binder.
// Submit / interrupt / clear stay on the owning pane.
func (l *InputLine) BindKeys(b KeyBinder) {
	b.BindKeyFunc("hist-prev", func(args ...any) { l.HistoryPrev() }, "<Up>", "<C-p>")
	b.BindKeyFunc("hist-next", func(args ...any) { l.HistoryNext() }, "<Down>", "<C-n>")
	b.BindKeyFunc("cursor-left", func(args ...any) { l.MoveLeft() }, "<Left>", "<C-b>")
	b.BindKeyFunc("cursor-right", func(args ...any) { l.MoveRight() }, "<Right>", "<C-f>")
	b.BindKeyFunc("cursor-home", func(args ...any) { l.MoveHome() }, "<Home>", "<C-a>")
	b.BindKeyFunc("cursor-end", func(args ...any) { l.MoveEnd() }, "<End>", "<C-e>")
	b.BindKeyFunc("backspace", func(args ...any) { l.Backspace() }, "<Backspace>", "<C-h>")
	b.BindKeyFunc("delete", func(args ...any) { l.DeleteForward() }, "<Delete>")
	b.BindKeyFunc("kill-line", func(args ...any) { l.KillToEnd() }, "<C-k>")
	b.BindKeyFunc("kill-bol", func(args ...any) { l.KillToStart() }, "<C-u>")
	b.BindKeyFunc("kill-word", func(args ...any) { l.KillWord() }, "<C-w>")
}

// Draw paints prompt + text at (x,y). Left-anchored: clip overflow on the right
// (no horizontal scroll). Returns caret column and the rune under it.
func (l *InputLine) Draw(c Canvas, x, y int, prompt string, promptStyle, textStyle tcell.Style) (cursorX int, under rune) {
	width := c.W()
	promptLen := len(prompt)
	for i, ch := range prompt {
		if x+i >= width {
			break
		}
		c.SetContent(x+i, y, ch, promptStyle)
	}
	under = ' '
	for i, ch := range l.text {
		col := x + promptLen + i
		if col >= width {
			break
		}
		c.SetContent(col, y, ch, textStyle)
		if i == l.cursor {
			under = ch
		}
	}
	if l.cursor < len(l.text) {
		under = rune(l.text[l.cursor])
	}
	cursorX = x + promptLen + l.cursor
	if width > 0 && cursorX >= width {
		cursorX = width - 1
		under = '…'
	}
	return cursorX, under
}

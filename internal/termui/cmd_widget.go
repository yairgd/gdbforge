package termui

import (
	"fmt"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
)

// CmdKind selects how the cmdline mux behaves after Activate / ActivateSearch.
type CmdKind int

const (
	CmdKindCommand CmdKind = iota // leading ':' — parse / complete / execute
	CmdKindSearch                 // leading '/' — live buffer search
)

type CmdWidget struct {
	BaseWidget

	history       *MemoryHistory
	searchHistory *MemoryHistory
	parser        *commands.CommandParser
	clipboard     ClipboardIO
	active        bool
	kind          CmdKind
	text          string
	cursor        int

	// onExecute runs a successfully parsed command (app controller).
	// When set, submit does not call parser.Execute in the view.
	onExecute func()
	// onChange runs after any edit while active (live /search preview).
	onChange func(text string)
	// onSearchSubmit runs on Enter in search kind with the pattern (no leading '/').
	onSearchSubmit func(pattern string)
	// postInterrupt queues UI events on the main loop (SubmitMsg, CompletionMsg, …).
	postInterrupt func(any)
}

func NewCmdWidget(reg *commands.CommandRegistry) *CmdWidget {
	return &CmdWidget{
		BaseWidget:    BaseWidget{cursor: NewNativeCursor()},
		history:       NewMemoryHistory(),
		searchHistory: NewMemoryHistory(),
		parser:        commands.NewCommandParser(reg),
		active:        false,
		kind:          CmdKindCommand,
	}
}

// CommandHistoryItems returns ':' cmdline history (oldest first).
func (c *CmdWidget) CommandHistoryItems() []string {
	if c == nil || c.history == nil {
		return nil
	}
	return c.history.Items()
}

// LoadCommandHistory replaces ':' cmdline history.
func (c *CmdWidget) LoadCommandHistory(items []string) {
	if c == nil || c.history == nil {
		return
	}
	c.history.Load(items)
}

// SearchHistoryItems returns '/' search cmdline history (oldest first).
func (c *CmdWidget) SearchHistoryItems() []string {
	if c == nil || c.searchHistory == nil {
		return nil
	}
	return c.searchHistory.Items()
}

// LoadSearchHistory replaces '/' search cmdline history.
func (c *CmdWidget) LoadSearchHistory(items []string) {
	if c == nil || c.searchHistory == nil {
		return
	}
	c.searchHistory.Load(items)
}

// SetOnExecute registers the Enter handler for a fully parsed command.
// The app should call ExecuteParsed from this callback.
func (c *CmdWidget) SetOnExecute(fn func()) {
	c.onExecute = fn
}

// SetOnChange registers a callback after cmdline text edits (search live preview).
func (c *CmdWidget) SetOnChange(fn func(text string)) {
	c.onChange = fn
}

// SetOnSearchSubmit registers the Enter handler for search mode.
func (c *CmdWidget) SetOnSearchSubmit(fn func(pattern string)) {
	c.onSearchSubmit = fn
}

// ExecuteParsed runs the last successfully Parse'd command action.
func (c *CmdWidget) ExecuteParsed() error {
	if c == nil || c.parser == nil || !c.parser.CanExecute() {
		return commands.ErrCommandNotExecutable
	}
	return c.parser.Execute()
}

// SetClipboard wires the shared copy/paste bridge (same as Viewport / ConsolePane).
func (c *CmdWidget) SetClipboard(io ClipboardIO) {
	c.clipboard = io
}

// Active reports whether the cmdline is editing (command / search / completion).
func (c *CmdWidget) Active() bool { return c.active }

// Kind reports whether the cmdline is in command or search mux mode.
func (c *CmdWidget) Kind() CmdKind { return c.kind }

// Text returns the current cmdline contents (including leading ':' or '/').
func (c *CmdWidget) Text() string { return c.text }

// Pattern returns the editable text after the leading prefix rune.
func (c *CmdWidget) Pattern() string {
	runes := []rune(c.text)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[1:])
}

func (c *CmdWidget) prefix() rune {
	if c.kind == CmdKindSearch {
		return '/'
	}
	return ':'
}

func (c *CmdWidget) hist() *MemoryHistory {
	if c.kind == CmdKindSearch {
		return c.searchHistory
	}
	return c.history
}

func (c *CmdWidget) SetPostInterrupt(fn func(any)) {
	c.postInterrupt = fn
}

func (c *CmdWidget) emit(ev SubmitMsg) {
	if c.postInterrupt != nil {
		c.postInterrupt(ev)
	}
}

func (c *CmdWidget) postCompletion(msg CompletionMsg) {
	if c.postInterrupt != nil {
		c.postInterrupt(msg)
	}
}

func (c *CmdWidget) notifyChange() {
	if c.kind == CmdKindSearch {
		if c.postInterrupt != nil {
			c.postInterrupt(SearchTextChangedMsg{Text: c.text})
		} else if c.onChange != nil {
			c.onChange(c.text)
		}
		return
	}
	if c.onChange != nil {
		c.onChange(c.text)
	}
}

func (c *CmdWidget) submitCommand() {
	line := strings.TrimSpace(c.text)
	if strings.HasPrefix(line, ":") {
		line = strings.TrimSpace(line[1:])
	}

	if line == "" {
		c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
		return
	}

	if err := c.parser.Parse(line); err != nil {
		c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
		return
	}

	if c.parser.CanExecute() {
		if c.onExecute != nil {
			c.onExecute()
			return
		}
		_ = c.parser.Execute()
		return
	}

	c.emit(SubmitMsg{Text: c.text, CmdID: CmdUnknown})
}

func (c *CmdWidget) syncParser() {
	line := c.text
	if strings.HasPrefix(line, ":") {
		line = line[1:]
	}
	c.parser.Sync(line, c.cursor-1)
}

func (c *CmdWidget) tokenBounds() (start, end int) {
	runes := []rune(c.text)
	end = c.cursor
	start = 1
	for i := 1; i < end; i++ {
		if runes[i] == ' ' {
			start = i + 1
		}
	}
	return start, end
}

func (c *CmdWidget) replaceToken(name string) {
	start, end := c.tokenBounds()
	runes := []rune(c.text)
	prefix := string(runes[:start])
	suffix := string(runes[end:])
	c.text = prefix + name + suffix
	c.cursor = len([]rune(prefix + name))
	c.notifyChange()
}

// ApplyCompletion replaces the current token with name (wildmenu accept).
func (c *CmdWidget) ApplyCompletion(name string) {
	if name == "" || c.kind != CmdKindCommand {
		return
	}
	c.replaceToken(name)
}

// CompletionNames returns Tab candidates for the current cmdline token.
func (c *CmdWidget) CompletionNames() []string {
	if c == nil || !c.active || c.kind != CmdKindCommand {
		return nil
	}
	c.syncParser()
	return c.parser.SuggestionNames()
}

// Activate opens command mode (leading ':').
func (c *CmdWidget) Activate() {
	c.active = true
	c.kind = CmdKindCommand
	c.text = ":"
	c.cursor = 1
}

// ActivateSearch opens search mode (leading '/').
func (c *CmdWidget) ActivateSearch() {
	c.active = true
	c.kind = CmdKindSearch
	c.text = "/"
	c.cursor = 1
}

func (c *CmdWidget) Deativate() {
	c.active = false
	c.text = ""
	c.cursor = 0
	c.kind = CmdKindCommand
	c.history.ResetNavigation()
	c.searchHistory.ResetNavigation()
}

// SetCursorAtLocalX places the caret from a click x relative to the cmdline left edge.
// Column 0 is the leading prefix; the caret never moves onto or before it.
func (c *CmdWidget) SetCursorAtLocalX(localX int) {
	if !c.active {
		return
	}
	n := len([]rune(c.text))
	if n < 1 {
		return
	}
	if localX < 1 {
		localX = 1
	}
	if localX > n {
		localX = n
	}
	c.cursor = localX
}

func (c *CmdWidget) HandleEvent(ev tcell.Event) {

	switch e := ev.(type) {

	case *tcell.EventResize:
		return

	case *tcell.EventKey:
		if !c.active {
			return
		}
		if isPasteKey(e) {
			c.pasteAtCursor()
			return
		}
		if isCopyCutKey(e) {
			// No mark selection on cmdline: copy/cut the editable text after prefix.
			c.copyEditable()
			if e.Key() == tcell.KeyCtrlX || (e.Modifiers()&tcell.ModCtrl != 0 && (e.Rune() == 'x' || e.Rune() == 'X')) {
				c.clearEditable()
			}
			return
		}
		if c.handleEditKey(e) {
			return
		}

		switch e.Key() {

		case tcell.KeyEscape:
			c.exitMode()
			return

		case tcell.KeyTAB:
			if c.kind != CmdKindCommand {
				return
			}
			c.syncParser()
			names := c.parser.SuggestionNames()
			token := c.parser.CurrentToken()
			restArgs := c.parser.CurrentIsRestArgs()

			c.postCompletion(CompletionMsg{
				Input: c.text,
				Token: token,
				Names: names,
			})

			if len(names) != 1 {
				return
			}

			c.replaceToken(names[0])
			if restArgs {
				return
			}
			// Unique tree match: add a trailing space when the command takes more input.
			suggestions := c.parser.Suggestions()
			if len(suggestions) == 1 {
				node := suggestions[0]
				children, _ := node.Complete("")
				if len(children) > 0 || node.RestArgs {
					c.text += " "
					c.cursor = len([]rune(c.text))
					// :b / :e / :layout — after accepting the command, immediately
					// show rest-arg candidates (buffers, files, layouts) so Tab
					// does not look like GDB -complete on an empty token.
					if node.RestArgs && node.CompleteArgs != nil {
						c.syncParser()
						rest := c.parser.SuggestionNames()
						c.postCompletion(CompletionMsg{
							Input: c.text,
							Token: c.parser.CurrentToken(),
							Names: rest,
						})
					}
				}
			}
			return

		case tcell.KeyEnter:
			c.hist().Add(c.text)
			c.hist().ResetNavigation()
			if c.kind == CmdKindSearch {
				pat := c.Pattern()
				if c.postInterrupt != nil {
					c.postInterrupt(SearchSubmittedMsg{Pattern: pat})
				} else if c.onSearchSubmit != nil {
					c.onSearchSubmit(pat)
				}
			} else {
				c.submitCommand()
			}

			c.active = false
			c.text = ""
			c.cursor = 0

			return

		case tcell.KeyUp:
			pfx := string(c.prefix())
			c.text = c.hist().Prev()
			if c.text != "" && !strings.HasPrefix(c.text, pfx) {
				c.text = pfx + c.text
			}
			c.cursor = len([]rune(c.text))
			c.notifyChange()
			return

		case tcell.KeyDown:
			pfx := string(c.prefix())
			c.text = c.hist().Next()
			if c.text != "" && !strings.HasPrefix(c.text, pfx) {
				c.text = pfx + c.text
			}
			c.cursor = len([]rune(c.text))
			c.notifyChange()
			return

		case tcell.KeyLeft:
			if c.cursor > 1 { // keep cursor after prefix
				c.cursor--
			}
			return

		case tcell.KeyRight:
			if c.cursor < len([]rune(c.text)) {
				c.cursor++
			}
			return

		case tcell.KeyBackspace, tcell.KeyBackspace2:
			r := []rune(c.text)
			pfx := c.prefix()

			// deleting prefix exits cmdline mode
			if len(r) == 1 && r[0] == pfx {
				c.exitMode()
				return
			}

			if c.cursor > 1 {
				r = append(r[:c.cursor-1], r[c.cursor:]...)
				c.cursor--
				c.text = string(r)
				c.notifyChange()
			}

			return

		default:
			ch := e.Rune()
			if ch == 0 {
				return
			}

			r := []rune(c.text)
			r = append(r, 0)
			copy(r[c.cursor+1:], r[c.cursor:])
			r[c.cursor] = ch
			c.text = string(r)
			c.cursor++
			c.notifyChange()
			return
		}
	case *tcell.EventClipboard:
		if !c.active {
			return
		}
		if data := e.Data(); len(data) > 0 {
			c.pasteText(string(data))
		}
	case *tcell.EventMouse:
		if !c.active {
			return
		}
		if isMiddlePaste(e) {
			c.pastePrimaryAtCursor()
			return
		}
		if e.Buttons()&tcell.ButtonPrimary != 0 {
			// Caller should prefer SetCursorAtLocalX with a known origin;
			// without a rect, treat absolute X as local (tests / simple hosts).
			x, _ := e.Position()
			c.SetCursorAtLocalX(x)
		}
	case *tcell.EventError:
	case tcell.EventFocus:
	case *tcell.EventInterrupt:
	case *tcell.EventPaste:
	case *tcell.EventTime:
	default:
		panic(fmt.Sprintf("unexpected tcell.Event: %#v", e))
	}
}

func (c *CmdWidget) exitMode() {
	c.emit(SubmitMsg{
		Text:  "exit from command mode",
		CmdID: CmdExitMode,
	})
	c.active = false
	c.text = ""
	c.cursor = 0
}

// handleEditKey handles readline-style editing chords on ':' and '/' cmdlines.
func (c *CmdWidget) handleEditKey(e *tcell.EventKey) bool {
	switch e.Key() {
	case tcell.KeyCtrlA:
		c.cursor = 1
		return true
	case tcell.KeyCtrlE:
		c.cursor = len([]rune(c.text))
		return true
	case tcell.KeyCtrlU:
		c.exitMode()
		return true
	}
	if e.Key() == tcell.KeyRune {
		r := e.Rune()
		if e.Modifiers()&tcell.ModCtrl != 0 {
			switch r {
			case 'a', 'A':
				c.cursor = 1
				return true
			case 'e', 'E':
				c.cursor = len([]rune(c.text))
				return true
			case 'u', 'U':
				c.exitMode()
				return true
			}
		}
		switch r {
		case 0x01: // Ctrl-A
			c.cursor = 1
			return true
		case 0x05: // Ctrl-E
			c.cursor = len([]rune(c.text))
			return true
		case 0x15: // Ctrl-U
			c.exitMode()
			return true
		}
	}
	return false
}

func (c *CmdWidget) pasteAtCursor() {
	c.pasteText(c.clipboard.pasteText())
}

func (c *CmdWidget) pastePrimaryAtCursor() {
	c.pasteText(c.clipboard.pastePrimaryText())
}

func (c *CmdWidget) pasteText(text string) {
	text = firstLinePaste(text)
	if text == "" || !c.active {
		return
	}
	runes := []rune(c.text)
	ins := []rune(text)
	out := make([]rune, 0, len(runes)+len(ins))
	out = append(out, runes[:c.cursor]...)
	out = append(out, ins...)
	out = append(out, runes[c.cursor:]...)
	c.text = string(out)
	c.cursor += len(ins)
	c.notifyChange()
}

func (c *CmdWidget) editableText() string {
	runes := []rune(c.text)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[1:])
}

func (c *CmdWidget) copyEditable() {
	c.clipboard.copyText(c.editableText())
}

func (c *CmdWidget) clearEditable() {
	if !c.active {
		return
	}
	c.text = string(c.prefix())
	c.cursor = 1
	c.notifyChange()
}

func (m *CmdWidget) Draw(c Canvas) {
	c.ClearLine(0, tcell.StyleDefault)

	if !m.active {
		return
	}

	// Left-anchored: never horizontal-scroll the cmdline; clip overflow on the right.
	width := c.W()
	runes := []rune(m.text)
	visible := runes
	if width > 0 && len(runes) > width {
		if width == 1 {
			visible = []rune("…")
		} else {
			visible = append(append([]rune{}, runes[:width-1]...), '…')
		}
	}
	c.Print(0, 0, tcell.StyleDefault, string(visible))

	cur := m.cursor
	if cur < 0 {
		cur = 0
	}
	under := ' '
	if cur < len(runes) {
		under = runes[cur]
	}
	paintX := cur
	if width > 0 && paintX >= width {
		paintX = width - 1
		under = '…'
	}
	m.Cursor().Paint(c, paintX, 0, under)
}

func (m *CmdWidget) DrawStatusLine(c Canvas, active bool) {}

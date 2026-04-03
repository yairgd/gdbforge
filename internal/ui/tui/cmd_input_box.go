package tui

import (
	"strings"

	"github.com/yairgd/promptcore/internal/core"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type CmdInputBox struct {
	ta textinput.Model

	history     core.History
	completer   core.AutoCompleter
	active      bool
	completions []string
	compIndex   int
}

type CancelCmdMode struct{}

func NewCmdInputBox(h core.History, c core.AutoCompleter) *CmdInputBox {

	ti := textinput.New()
	ti.Placeholder = "type ESC + \":\" to use vim style commands"
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 0

        ti.TextStyle = cmdInputBoxStyle
	ti.PlaceholderStyle = cmdInputBoxStyle

	return &CmdInputBox{
		ta:        ti,
		history:   h,
		completer: c,
		active:    false,
	}
}

func (i *CmdInputBox) Update(msg tea.Msg) tea.Cmd {

	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.Type {

		// --- ENTER ---
		case tea.KeyEnter:
			text := i.ta.Value()

			i.history.Add(text)
			i.history.ResetNavigation()

			i.ta.Reset()

			return func() tea.Msg {
				return core.RunCommand{Command: text}
			}

		// --- HISTORY ---
		case tea.KeyUp:
			i.ta.SetValue(i.history.Prev())
			i.ta.CursorEnd()
			return nil

		case tea.KeyDown:
			i.ta.SetValue(i.history.Next())
			i.ta.CursorEnd()
			return nil

		// --- AUTOCOMPLETE ---
		case tea.KeyTab:

			current := strings.TrimPrefix(i.ta.Value(), ":")

			if i.completions == nil {
				i.completions = i.completer.Complete(current)
				i.compIndex = 0
			} else {
				i.compIndex = (i.compIndex + 1) % len(i.completions)
			}

			if len(i.completions) > 0 {
				i.ta.SetValue(":" + i.completions[i.compIndex])
				i.ta.CursorEnd()
			}

			return nil

		// --- ESC ---
		case tea.KeyEsc:
			i.ta.Reset()
			return func() tea.Msg {
				return CancelCmdMode{}
			}
		}
	}

	// sync buffer to history
	i.history.SetBuffer(i.ta.Value())

	var cmd tea.Cmd
	i.ta, cmd = i.ta.Update(msg)
	return cmd
}

// --- view ---

func (i *CmdInputBox) View() string {
	return i.ta.View()
}

// --- misc ---

func (i *CmdInputBox) Reset() {
	i.ta.Reset()
	i.history.ResetNavigation()
}

func (i *CmdInputBox) SetActive() {
	i.active = true
	i.ta.SetValue(":")
	i.history.ResetNavigation()
}

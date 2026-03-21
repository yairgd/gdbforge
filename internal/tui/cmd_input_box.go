package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/promptcore/internal/events"
)

type CmdInputBox struct {
	ta          textinput.Model
	systemState string
	active      bool

	history    []string
	historyPos int

	completions []string
	compIndex   int
}

type CancelCmdMode struct{}
type EnterCmdMode struct{}

var commandList = []string{
	"break",
	"continue",
	"next",
	"step",
	"print",
	"bt",
	"info",
	"run",
	"quit",
}

func NewCmdInputBox() *CmdInputBox {

	ti := textinput.New()

	ti.Placeholder = "type ESC + \":\" to use vim style commands"
	ti.SetValue("")
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 0

	ti.TextStyle = cmdInputBoxStyle
	ti.PlaceholderStyle = cmdInputBoxStyle

	return &CmdInputBox{
		ta:     ti,
		active: false,
	}

}

func (i *CmdInputBox) Update(msg tea.Msg) tea.Cmd {

	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.Type {

		case tea.KeyEnter:

			text := i.ta.Value()

			if text != "" {
				i.history = append(i.history, text)
				i.historyPos = len(i.history)
			}

			i.completions = nil
			i.compIndex = 0

			i.ta.Reset()

			return func() tea.Msg {
				return events.RunCommand{Command: text}
			}

		case tea.KeyUp:

			if len(i.history) == 0 {
				break
			}

			if i.historyPos > 0 {
				i.historyPos--
				i.ta.SetValue(i.history[i.historyPos])
				i.ta.CursorEnd()
			}

			return nil

		case tea.KeyDown:

			if len(i.history) == 0 {
				break
			}

			if i.historyPos < len(i.history)-1 {
				i.historyPos++
				i.ta.SetValue(i.history[i.historyPos])
			} else {
				i.historyPos = len(i.history)
				i.ta.SetValue("")
			}

			i.ta.CursorEnd()
			return nil

		case tea.KeyTab:

			current := strings.TrimPrefix(i.ta.Value(), ":")

			if i.completions == nil {
				i.completions = findMatches(current)
				i.compIndex = 0
			} else {
				i.compIndex++
				if i.compIndex >= len(i.completions) {
					i.compIndex = 0
				}
			}

			if len(i.completions) > 0 {
				i.ta.SetValue(":" + i.completions[i.compIndex])
				i.ta.CursorEnd()
			}

			return nil

		case tea.KeyEsc:

			i.ta.Reset()

			return func() tea.Msg {
				return CancelCmdMode{}
			}

		}
	}

	var cmd tea.Cmd
	i.ta, cmd = i.ta.Update(msg)
	return cmd

}

func findMatches(prefix string) []string {

	var matches []string

	for _, c := range commandList {
		if strings.HasPrefix(c, prefix) {
			matches = append(matches, c)
		}
	}

	return matches

}

func (i *CmdInputBox) View() string {
	return i.ta.View()
}

func (i *CmdInputBox) Reset() {
	i.ta.Reset()
}

func (i *CmdInputBox) SetWidth(w int) {}

func (i *CmdInputBox) SetHeight(h int) {}

func (i *CmdInputBox) SetActive() {
	i.active = true
	i.ta.SetValue(":")
	i.historyPos = len(i.history)
}

func (i *CmdInputBox) SetSystemStateText(text string) {
	i.systemState = text
}

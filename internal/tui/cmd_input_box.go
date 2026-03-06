package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type CmdInputBox struct {
	ta          textinput.Model
	systemState string
	active      bool
}

type SubmitCmdMsg struct {
	Text string
}

type CancelCmdMode struct{}
type EnterCmdMode struct{}

func NewCmdInputBox() *CmdInputBox {

	ti := textinput.New()

	ti.Placeholder = "use vim style command"
	ti.SetValue("")
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 0

	ti.TextStyle = cmdInputBoxStyle
	ti.PlaceholderStyle = cmdInputBoxStyle

	//	ta.SetHeight(1)

	//	ta.ShowLineNumbers = false
	//	ta.Prompt = ""

	//	ta.FocusedStyle.Base = cmdInputBoxStyle
	//	ta.BlurredStyle.Base = cmdInputBoxStyle

	return &CmdInputBox{ta: ti, active: false}
}
func (i *CmdInputBox) Update(msg tea.Msg) tea.Cmd {

	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.Type {

		case tea.KeyEnter:
			i.active = true
			text := i.ta.Value()
			i.ta.Reset()

			return func() tea.Msg {
				return SubmitCmdMsg{Text: text}
			}

		case tea.KeyEsc:
			if i.active {
				i.ta.Reset()

				return func() tea.Msg {
					return CancelCmdMode{}
				}
			} else {
				i.active = false
			}
		}
	}

	var cmd tea.Cmd
	i.ta, cmd = i.ta.Update(msg)
	return cmd
}

//func (i *CmdInputBox) View(width int) string {
//	v := i.ta.View()

//	return cmdInputBoxStyle.
//		Width(width).
//		Render(v)
//}

func (i *CmdInputBox) View() string {
	return i.ta.View()
}

func (i *CmdInputBox) Reset() {
	i.ta.Reset()
}

func (i *CmdInputBox) SetWidth(w int) {
	// i.ta.SetWidth(w)
}

func (i *CmdInputBox) SetHeight(h int) {
	// i.ta.SetHeight(h)
}

func (i *CmdInputBox) SetActive() {
	i.active = true
	i.ta.SetValue(":")

}

func (i *CmdInputBox) SetSystemStateText(text string) {
	i.systemState = text

}

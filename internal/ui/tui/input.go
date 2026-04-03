package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/promptcore/internal/core"
)

//
// ===================== INPUT BOX COMPONENT =====================
// Encapsulates textarea completely.
//

type InputBox struct {
	ta textarea.Model
}

//
// Constructor (recommended)
//

func NewInputBox() InputBox {
	ta := textarea.New()
	ta.Placeholder = "Write multi-line text here..."
	ta.Focus()
	ta.CharLimit = 0
	//ta.SetHeight(6)
	ta.ShowLineNumbers = true

	return InputBox{ta: ta}
}

//
// Update handles ONLY textarea behavior.
//

func (i *InputBox) Update(msg tea.KeyMsg) tea.Cmd {

	switch msg.Type {

	case tea.KeyPgUp:
		for j := 0; j < 10; j++ {
			i.ta, _ = i.ta.Update(tea.KeyMsg{Type: tea.KeyUp})
		}
		return nil

	case tea.KeyPgDown:
		for j := 0; j < 10; j++ {
			i.ta, _ = i.ta.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		return nil

	case tea.KeyCtrlS:
		text := i.ta.Value()
		i.ta.Reset()

		return func() tea.Msg {
			return core.SubmitMsg{Text: text}
		}
	}
	var cmd tea.Cmd
	i.ta, cmd = i.ta.Update(msg)
	return cmd
}

//
// Public API
//

func (i *InputBox) View() string {
	return i.ta.View()
}

func (i *InputBox) Value() string {
	return i.ta.Value()
}

func (i *InputBox) Reset() {
	i.ta.Reset()
}

func (i *InputBox) Blur() {
	i.ta.Blur()
}

func (i *InputBox) Focus() {
	i.ta.Focus()
}

func (i *InputBox) SetWidth(w int) {
	i.ta.SetWidth(w)
}

func (i *InputBox) SetHeight(h int) {
	i.ta.SetHeight(h)
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yairgd/promptcore/internal/core"
)

// emit event as tea.Cmd
func emitEvent(e core.Event) tea.Cmd {
	return func() tea.Msg {
		return e
	}
}

// simulate async work
func sendMessageCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return core.MessageSent{Text: text}
	}
}

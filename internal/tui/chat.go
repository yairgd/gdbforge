package tui

import (
	"fmt"
)

func (m *Model) refreshViewport() {
	content := ""
	for i, l := range m.state.Lines {
		content += fmt.Sprintf("[%d]\n%s\n\n", i+1, l)
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

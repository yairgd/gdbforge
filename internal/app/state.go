package app

import "strings"

type State struct {
	Mode         Mode
	Lines        []string
	CommandInput string
	Width        int
	Height       int
}

func NewState() State {
	return State{
		Mode:  InsertMode,
		Lines: []string{},
	}
}

func (s *State) SubmitText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	s.Lines = append(s.Lines, text)
}

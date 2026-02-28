package app

import "strings"

func (s *State) SubmitText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	s.Lines = append(s.Lines, text)
}

func (s *State) ExecuteCommand() (quit bool) {
	cmd := strings.TrimSpace(s.CommandInput)

	switch cmd {
	case "q":
		return true
	case "hello":
		s.Lines = append(s.Lines, "🤖 hi this is hello command")
	case "clear":
		s.Lines = []string{}
	default:
		s.Lines = append(s.Lines, "Unknown command: "+cmd)
	}

	s.CommandInput = ""
	return false
}

package app

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

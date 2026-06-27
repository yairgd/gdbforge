package cgdb

type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeSearch
	ModeInsert
)

type AppState struct {
	mode Mode
}

func (a *AppState) Mode() Mode {
	return a.mode
}

func (a *AppState) SetMode(mode Mode) {
	a.mode = mode
}

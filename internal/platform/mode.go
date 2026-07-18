package platform

import "sync"

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
)

// PTYOwner identifies which frontend currently holds exclusive PTY write
// intent (the write mutex still enforces exclusivity in ptyx).
type PTYOwner int

const (
	PTYOwnerNone PTYOwner = iota
	PTYOwnerUI              // GDB / Exec console submit
	PTYOwnerMCP             // GdbMcpService / :AI tools
)

func (o PTYOwner) String() string {
	switch o {
	case PTYOwnerUI:
		return "ui"
	case PTYOwnerMCP:
		return "mcp"
	default:
		return "none"
	}
}

// AppState is the process-global interaction and system state for a TermApp
// session: input mode, who owns the PTY write path, and layout policy.
type AppState struct {
	mu sync.RWMutex

	mode Mode

	// ptyOwner is who currently intends to write the shared debugger/exec PTY.
	ptyOwner PTYOwner

	// equalAlways mirrors Vim 'equalalways': keep split panes evenly sized
	// after splits / layout rebuilds (user drag ratios are reset when true).
	equalAlways bool
}

func (a *AppState) Mode() Mode {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mode
}

func (a *AppState) SetMode(mode Mode) {
	a.mu.Lock()
	a.mode = mode
	a.mu.Unlock()
}

func (a *AppState) PTYOwner() PTYOwner {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ptyOwner
}

func (a *AppState) SetPTYOwner(owner PTYOwner) {
	a.mu.Lock()
	a.ptyOwner = owner
	a.mu.Unlock()
}

// WithPTYOwner sets owner for the duration of fn, then restores previous.
func (a *AppState) WithPTYOwner(owner PTYOwner, fn func()) {
	a.mu.Lock()
	prev := a.ptyOwner
	a.ptyOwner = owner
	a.mu.Unlock()
	defer a.SetPTYOwner(prev)
	fn()
}

func (a *AppState) EqualAlways() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.equalAlways
}

func (a *AppState) SetEqualAlways(v bool) {
	a.mu.Lock()
	a.equalAlways = v
	a.mu.Unlock()
}

package platform

import "sync"

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
	ModeCompletion // wildmenu: only CompletionBar receives nav keys
)

// PTYOwner identifies which frontend currently holds exclusive PTY write
// intent (the write mutex still enforces exclusivity in ptyx).
type PTYOwner int

const (
	PTYOwnerNone PTYOwner = iota
	PTYOwnerUI              // GDB / Exec console submit
	PTYOwnerMCP             // GdbMcpService / :AI tools
	PTYOwnerApp             // App silent MI queries (file list, etc.)
)

func (o PTYOwner) String() string {
	switch o {
	case PTYOwnerUI:
		return "ui"
	case PTYOwnerMCP:
		return "mcp"
	case PTYOwnerApp:
		return "app"
	default:
		return "none"
	}
}

// AppState is the process-global session model: interaction mode, PTY mux
// owner, layout policy, and debugger source location / file list.
type AppState struct {
	mu sync.RWMutex

	mode Mode

	// ptyOwner is who currently intends to write the shared debugger/exec PTY.
	ptyOwner PTYOwner

	// equalAlways mirrors Vim 'equalalways': keep split panes evenly sized
	// after splits / layout rebuilds (user drag ratios are reset when true).
	equalAlways bool

	sourceFiles []string
	currentFile string
	currentLine int // 1-based; 0 = unset
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

func (a *AppState) SourceFiles() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, len(a.sourceFiles))
	copy(out, a.sourceFiles)
	return out
}

func (a *AppState) SetSourceFiles(files []string) {
	a.mu.Lock()
	a.sourceFiles = append([]string(nil), files...)
	a.mu.Unlock()
}

func (a *AppState) CurrentFile() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentFile
}

func (a *AppState) CurrentLine() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentLine
}

// SetCurrentLocation sets the PC / stop file (1-based line).
func (a *AppState) SetCurrentLocation(file string, line int) {
	a.mu.Lock()
	a.currentFile = file
	a.currentLine = line
	a.mu.Unlock()
}

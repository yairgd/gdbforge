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
	PTYOwnerApp             // Silent MI: BreakpointWidget, -break-list, file list, …
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

const LayoutDefault = "default"

// DefaultLayoutRatios holds split shares for NewTabDefaultDebugLayout.
type DefaultLayoutRatios struct {
	// Left is the outer vertical First share (Code+GDB column). Default 2/3.
	Left float64
	// Output is the right-column First share (Output pane height). Default 1/2.
	Output float64
	// BottomFirst is Breakpoints' share of the bottom half of the right column.
	// Threads/Call stack split the rest equally. Default 1/3.
	BottomFirst float64
}

// AppState is the process-global session model: interaction mode, PTY mux
// owner, layout policy, and debugger source location / file list.
type AppState struct {
	mu sync.RWMutex

	mode Mode

	// ptyOwner is who currently intends to write the shared debugger/exec PTY.
	ptyOwner PTYOwner

	// equalAlways mirrors Vim 'equalalways': rebalance split ratios after
	// splits / closes (not on every paint). User drag ratios are kept until
	// the next structural change when true.
	equalAlways bool

	// defaultLayoutRatios are the preset splits for :layout default.
	defaultLayoutRatios DefaultLayoutRatios

	// clearOutput mirrors Vim 'clearoutput': clear the Output pane when the
	// GDB session starts (default true). Stepping (^running) does not clear.
	clearOutput bool

	// inferiorRunning is true between ^running and the next *stopped / exit.
	inferiorRunning bool

	layouts       []string
	currentLayout string

	sourceFiles []string
	currentFile string
	currentLine int // 1-based; 0 = unset
}

// NewAppState returns AppState with Vim-like defaults.
func NewAppState() *AppState {
	return &AppState{
		equalAlways: true,
		defaultLayoutRatios: DefaultLayoutRatios{
			Left:        2.0 / 3.0,
			Output:      1.0 / 2.0,
			BottomFirst: 1.0 / 3.0,
		},
		clearOutput:   true,
		layouts:       []string{LayoutDefault},
		currentLayout: LayoutDefault,
	}
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

func (a *AppState) LayoutLeftRatio() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.defaultLayoutRatios.Left
}

// SetLayoutLeftRatio sets the default-layout left column share (clamped to [0.1, 0.9]).
func (a *AppState) SetLayoutLeftRatio(r float64) {
	r = clampLayoutRatio(r)
	a.mu.Lock()
	a.defaultLayoutRatios.Left = r
	a.mu.Unlock()
}

// DefaultLayoutRatios returns the preset splits for :layout default.
func (a *AppState) DefaultLayoutRatios() DefaultLayoutRatios {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.defaultLayoutRatios
}

// SetDefaultLayoutRatios replaces the default-layout split presets (each clamped).
func (a *AppState) SetDefaultLayoutRatios(r DefaultLayoutRatios) {
	r.Left = clampLayoutRatio(r.Left)
	r.Output = clampLayoutRatio(r.Output)
	r.BottomFirst = clampLayoutRatio(r.BottomFirst)
	a.mu.Lock()
	a.defaultLayoutRatios = r
	a.mu.Unlock()
}

func clampLayoutRatio(r float64) float64 {
	if r < 0.1 {
		return 0.1
	}
	if r > 0.9 {
		return 0.9
	}
	return r
}

func (a *AppState) ClearOutput() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.clearOutput
}

func (a *AppState) SetClearOutput(v bool) {
	a.mu.Lock()
	a.clearOutput = v
	a.mu.Unlock()
}

func (a *AppState) InferiorRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.inferiorRunning
}

func (a *AppState) SetInferiorRunning(v bool) {
	a.mu.Lock()
	a.inferiorRunning = v
	a.mu.Unlock()
}

func (a *AppState) Layouts() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, len(a.layouts))
	copy(out, a.layouts)
	return out
}

func (a *AppState) RegisterLayout(name string) {
	if name == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, n := range a.layouts {
		if n == name {
			return
		}
	}
	a.layouts = append(a.layouts, name)
}

func (a *AppState) HasLayout(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, n := range a.layouts {
		if n == name {
			return true
		}
	}
	return false
}

func (a *AppState) CurrentLayout() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentLayout
}

func (a *AppState) SetCurrentLayout(name string) {
	a.mu.Lock()
	a.currentLayout = name
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

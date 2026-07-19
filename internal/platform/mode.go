package platform

import (
	"strings"
	"sync"

	tcell "github.com/gdamore/tcell/v2"
)

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

// DefaultLayoutRatios holds split shares for the default debugger workspace.
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

	// continueAfterClear: when the inferior is running and a breakpoint is
	// removed (clear / -break-delete), interrupt then optionally resume.
	// Default false — stay stopped after removing a breakpoint.
	continueAfterClear bool

	// inferiorRunning is true between ^running and the next *stopped / exit.
	inferiorRunning bool

	layouts       []string
	currentLayout string

	sourceFiles []string
	currentFile string
	currentLine int // 1-based; 0 = unset

	// markColor is the selected-row background for list pickers when focused
	// (e.g. :edit, callstack, breakpoints). Default blue; :set markcolor <name>.
	markColor tcell.Color
	// markDimColor is the selected-row background when the list pane is not
	// focused. Default gray; :set markdimcolor <name>.
	markDimColor tcell.Color

	// breakColor is the enabled-breakpoint background (CodeWidget gutter +
	// BreakpointWidget rows). Default red; :set breakcolor <name>.
	breakColor tcell.Color
	// breakDisabledColor is the disabled-breakpoint background. Default yellow;
	// :set breakdisabledcolor <name>.
	breakDisabledColor tcell.Color

	// escToCode: Esc leaves insert and focuses the CodeWidget leaf (default true).
	// :set esctocode / :set noesctocode.
	escToCode bool

	// breakMain: insert "break main" when the GDB session starts (default true).
	// :set breakmain / :set nobreakmain.
	breakMain bool

	// gdbListenPrint: when true, paint GDB console lines from App/MCP (listener)
	// traffic. Default true; :set gdblistenprint / :set nogdblistenprint.
	gdbListenPrint bool

	// gdbConsoleSilent is sticky: set true on App/MCP WithPTYOwner, cleared on
	// UI WithPTYOwner. Used with gdbListenPrint to suppress listener paint
	// after short Send() windows (owner restores to none before replies arrive).
	gdbConsoleSilent bool
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
		clearOutput:        true,
		layouts:            []string{LayoutDefault},
		currentLayout:      LayoutDefault,
		markColor:          tcell.ColorBlue,
		markDimColor:       tcell.ColorGray,
		breakColor:         tcell.ColorRed,
		breakDisabledColor: tcell.ColorYellow,
		escToCode:          true,
		breakMain:          true,
		gdbListenPrint:     true,
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
// UI ownership clears the sticky console-silent flag; App/MCP set it so
// async PTY replies after Send are still treated as listener traffic.
func (a *AppState) WithPTYOwner(owner PTYOwner, fn func()) {
	a.mu.Lock()
	prev := a.ptyOwner
	a.ptyOwner = owner
	switch owner {
	case PTYOwnerUI:
		a.gdbConsoleSilent = false
	case PTYOwnerApp, PTYOwnerMCP:
		a.gdbConsoleSilent = true
	}
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

func (a *AppState) ContinueAfterClear() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.continueAfterClear
}

func (a *AppState) SetContinueAfterClear(v bool) {
	a.mu.Lock()
	a.continueAfterClear = v
	a.mu.Unlock()
}

// EscToCode reports whether Esc focuses the CodeWidget leaf (default true).
func (a *AppState) EscToCode() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.escToCode
}

func (a *AppState) SetEscToCode(v bool) {
	a.mu.Lock()
	a.escToCode = v
	a.mu.Unlock()
}

// BreakMain reports whether to insert "break main" on GDB session start (default true).
func (a *AppState) BreakMain() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.breakMain
}

func (a *AppState) SetBreakMain(v bool) {
	a.mu.Lock()
	a.breakMain = v
	a.mu.Unlock()
}

// GdbListenPrint reports whether App/MCP PTY replies paint in the GDB console
// (default true).
func (a *AppState) GdbListenPrint() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.gdbListenPrint
}

func (a *AppState) SetGdbListenPrint(v bool) {
	a.mu.Lock()
	a.gdbListenPrint = v
	a.mu.Unlock()
}

// SuppressGdbConsole is true when the GDB widget should not paint DisplayLines
// (listener traffic and sticky silent after App/MCP writes, unless GdbListenPrint).
func (a *AppState) SuppressGdbConsole() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.gdbListenPrint {
		return false
	}
	if a.ptyOwner == PTYOwnerApp || a.ptyOwner == PTYOwnerMCP {
		return true
	}
	return a.gdbConsoleSilent
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

func (a *AppState) MarkColor() tcell.Color {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.markColor == tcell.ColorDefault {
		return tcell.ColorBlue
	}
	return a.markColor
}

func (a *AppState) SetMarkColor(c tcell.Color) {
	a.mu.Lock()
	a.markColor = c
	a.mu.Unlock()
}

func (a *AppState) MarkDimColor() tcell.Color {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.markDimColor == tcell.ColorDefault {
		return tcell.ColorGray
	}
	return a.markDimColor
}

func (a *AppState) SetMarkDimColor(c tcell.Color) {
	a.mu.Lock()
	a.markDimColor = c
	a.mu.Unlock()
}

func (a *AppState) BreakColor() tcell.Color {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.breakColor == tcell.ColorDefault {
		return tcell.ColorRed
	}
	return a.breakColor
}

func (a *AppState) SetBreakColor(c tcell.Color) {
	a.mu.Lock()
	a.breakColor = c
	a.mu.Unlock()
}

func (a *AppState) BreakDisabledColor() tcell.Color {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.breakDisabledColor == tcell.ColorDefault {
		return tcell.ColorYellow
	}
	return a.breakDisabledColor
}

func (a *AppState) SetBreakDisabledColor(c tcell.Color) {
	a.mu.Lock()
	a.breakDisabledColor = c
	a.mu.Unlock()
}

// ParseColorName maps a user color name to a tcell.Color (case-insensitive).
func ParseColorName(name string) (tcell.Color, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "blue":
		return tcell.ColorBlue, true
	case "darkblue":
		return tcell.ColorDarkBlue, true
	case "navy":
		return tcell.ColorNavy, true
	case "black":
		return tcell.ColorBlack, true
	case "gray", "grey":
		return tcell.ColorGray, true
	case "white":
		return tcell.ColorWhite, true
	case "red":
		return tcell.ColorRed, true
	case "green":
		return tcell.ColorGreen, true
	case "yellow":
		return tcell.ColorYellow, true
	case "cyan", "aqua":
		return tcell.ColorAqua, true
	case "magenta":
		return tcell.ColorPurple, true
	default:
		return tcell.ColorDefault, false
	}
}

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
	ModeSearch     // '/' cmdline: live buffer search (muxed with ModeCommand on CmdWidget)
	ModeCompletion // wildmenu: CompletionMenu + CompletionView (bar) receive keys
	ModeLua        // LuaWidget owns keys (Esc leaves)
)

// PTYOwner identifies which frontend currently holds exclusive PTY write
// intent (the write mutex still enforces exclusivity in ptyx).
type PTYOwner int

const (
	PTYOwnerNone PTYOwner = iota
	PTYOwnerUI            // GDB / Exec console submit
	PTYOwnerMCP           // GdbMcpService / :AI tools
	PTYOwnerApp           // Silent MI: BreakpointWidget, -break-list, file list, …
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

// AppState is the process-global UI session model: interaction mode, PTY mux
// owner, layout policy, and generic chrome colors. Debugger-specific state
// (stop location, BP colors, GDB flags) lives in gdbforge/debugstate.
type AppState struct {
	mu sync.RWMutex

	mode Mode

	// ptyOwner is who currently intends to write the shared debugger/exec PTY.
	ptyOwner PTYOwner

	// onPTYOwner is invoked whenever the PTY owner changes (WithPTYOwner / SetPTYOwner).
	// Used by app-private debugstate for console-silent sticky flags.
	onPTYOwner func(PTYOwner)

	// equalAlways mirrors Vim 'equalalways': rebalance split ratios after
	// splits / closes (not on every paint). User drag ratios are kept until
	// the next structural change when true.
	equalAlways bool

	// defaultLayoutRatios are the preset splits for :layout default.
	defaultLayoutRatios DefaultLayoutRatios

	layouts       []string
	currentLayout string

	// markColor is the selected-row background for list pickers when focused
	// (e.g. :edit, callstack, breakpoints). Default DefaultMarkColor; :set markcolor.
	markColor tcell.Color
	// markDimColor is the selected-row background when the list pane is not
	// focused. Default DefaultMarkDimColor; :set markdimcolor.
	markDimColor tcell.Color

	// codeSelColor is the focused CodeWidget selection row when not on PC.
	// Default DefaultCodeSelColor; :set codeselcolor.
	codeSelColor tcell.Color
	// mutedColor is dim/empty-list foreground. Default DefaultMutedColor;
	// :set mutedcolor.
	mutedColor tcell.Color
	// searchColor is the /search match background. Default DefaultSearchColor;
	// :set searchcolor.
	searchColor tcell.Color

	// escToCode: Esc leaves insert and focuses the CodeWidget leaf (default true).
	// :set esctocode / :set noesctocode.
	escToCode bool
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
		layouts:       []string{LayoutDefault},
		currentLayout: LayoutDefault,
		markColor:     DefaultMarkColor,
		markDimColor:  DefaultMarkDimColor,
		codeSelColor:  DefaultCodeSelColor,
		mutedColor:    DefaultMutedColor,
		searchColor:   DefaultSearchColor,
		escToCode:     true,
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
	hook := a.onPTYOwner
	a.mu.Unlock()
	if hook != nil {
		hook(owner)
	}
}

// SetPTYOwnerHook registers a callback for PTY owner changes (app debugstate).
func (a *AppState) SetPTYOwnerHook(fn func(PTYOwner)) {
	a.mu.Lock()
	a.onPTYOwner = fn
	a.mu.Unlock()
}

// WithPTYOwner sets owner for the duration of fn, then restores previous.
func (a *AppState) WithPTYOwner(owner PTYOwner, fn func()) {
	a.mu.Lock()
	prev := a.ptyOwner
	a.ptyOwner = owner
	hook := a.onPTYOwner
	a.mu.Unlock()
	if hook != nil {
		hook(owner)
	}
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

func (a *AppState) MarkColor() tcell.Color {
	return a.getColor(&a.markColor, DefaultMarkColor)
}

func (a *AppState) SetMarkColor(c tcell.Color) {
	a.setColor(&a.markColor, c)
}

func (a *AppState) MarkDimColor() tcell.Color {
	return a.getColor(&a.markDimColor, DefaultMarkDimColor)
}

func (a *AppState) SetMarkDimColor(c tcell.Color) {
	a.setColor(&a.markDimColor, c)
}

func (a *AppState) CodeSelColor() tcell.Color {
	return a.getColor(&a.codeSelColor, DefaultCodeSelColor)
}

func (a *AppState) SetCodeSelColor(c tcell.Color) {
	a.setColor(&a.codeSelColor, c)
}

func (a *AppState) MutedColor() tcell.Color {
	return a.getColor(&a.mutedColor, DefaultMutedColor)
}

func (a *AppState) SetMutedColor(c tcell.Color) {
	a.setColor(&a.mutedColor, c)
}

func (a *AppState) SearchColor() tcell.Color {
	return a.getColor(&a.searchColor, DefaultSearchColor)
}

func (a *AppState) SetSearchColor(c tcell.Color) {
	a.setColor(&a.searchColor, c)
}

func (a *AppState) getColor(field *tcell.Color, def tcell.Color) tcell.Color {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if *field == tcell.ColorDefault {
		return def
	}
	return *field
}

func (a *AppState) setColor(field *tcell.Color, c tcell.Color) {
	a.mu.Lock()
	*field = c
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
	case "silver":
		return tcell.ColorSilver, true
	case "white":
		return tcell.ColorWhite, true
	case "red":
		return tcell.ColorRed, true
	case "green":
		return tcell.ColorGreen, true
	case "yellow":
		return tcell.ColorYellow, true
	case "orange":
		return tcell.ColorOrange, true
	case "darkorange":
		return tcell.ColorDarkOrange, true
	case "cyan", "aqua":
		return tcell.ColorAqua, true
	case "magenta", "purple":
		return tcell.ColorPurple, true
	case "fuchsia":
		return tcell.ColorFuchsia, true
	case "darkslategray", "darkslategrey", "slategray", "slategrey":
		return tcell.ColorDarkSlateGray, true
	case "teal":
		return tcell.ColorTeal, true
	default:
		return tcell.ColorDefault, false
	}
}

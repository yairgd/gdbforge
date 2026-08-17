// Package debugstate holds debugger-session state that must not live in the
// reusable platform.AppState (framework). Wired by cmd/gdbforge.
package debugstate

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

// State is the gdbforge-private session model: inferior location, BP/PC
// colors, and GDB console paint policy.
type State struct {
	mu sync.RWMutex

	app *platform.AppState // mark / muted / code-sel colors + PTY owner reads

	clearOutput        bool
	continueAfterClear bool
	inferiorRunning    bool
	// stopUISuppress skips the next stop UI paint (Code ━━▶ / blue snap).
	// Armed when SendCmd Ctrl-C + continue for break/clear while running.
	stopUISuppress int

	sourceFiles []string
	currentFile string
	currentLine int
	stopFile    string
	stopLine    int

	breakColor         tcell.Color
	breakDisabledColor tcell.Color
	breakCondColor     tcell.Color
	pcColor            tcell.Color
	stackBreakColor    tcell.Color

	breakMain        bool
	gdbListenPrint   bool
	gdbTargetPrint   bool
	gdbConsoleSilent bool
	// kgdbMode: kernel kgdb over slow serial — CLI stepping, lighter post-stop MI.
	kgdbMode bool
	// skipKgdbAttachStackRefresh: skip one -stack-list-frames after target remote
	// on slow serial (frame 0 from *stopped is enough; full stack on next stop).
	skipKgdbAttachStackRefresh bool
}

// New returns DebugState with Vim-like debugger defaults.
// app may be nil; MarkColor accessors then use platform defaults.
func New(app *platform.AppState) *State {
	s := &State{
		app:                app,
		clearOutput:        true,
		breakColor:         platform.DefaultBreakColor,
		breakDisabledColor: platform.DefaultBreakDisabledColor,
		breakCondColor:     platform.DefaultBreakCondColor,
		pcColor:            platform.DefaultPCColor,
		stackBreakColor:    platform.DefaultStackBreakColor,
		breakMain:          true,
		gdbListenPrint:     true,
		gdbTargetPrint:     false,
	}
	if app != nil {
		app.SetPTYOwnerHook(s.notePTYOwner)
	}
	return s
}

func (s *State) notePTYOwner(owner platform.PTYOwner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch owner {
	case platform.PTYOwnerUI:
		s.gdbConsoleSilent = false
	case platform.PTYOwnerApp, platform.PTYOwnerMCP:
		s.gdbConsoleSilent = true
	}
}

func (s *State) ClearOutput() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clearOutput
}

func (s *State) SetClearOutput(v bool) {
	s.mu.Lock()
	s.clearOutput = v
	s.mu.Unlock()
}

func (s *State) ContinueAfterClear() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.continueAfterClear
}

func (s *State) SetContinueAfterClear(v bool) {
	s.mu.Lock()
	s.continueAfterClear = v
	s.mu.Unlock()
}

func (s *State) BreakMain() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.breakMain
}

func (s *State) SetBreakMain(v bool) {
	s.mu.Lock()
	s.breakMain = v
	s.mu.Unlock()
}

func (s *State) GdbListenPrint() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gdbListenPrint
}

func (s *State) SetGdbListenPrint(v bool) {
	s.mu.Lock()
	s.gdbListenPrint = v
	s.mu.Unlock()
}

func (s *State) GdbTargetPrint() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gdbTargetPrint
}

func (s *State) SetGdbTargetPrint(v bool) {
	s.mu.Lock()
	s.gdbTargetPrint = v
	s.mu.Unlock()
}

func (s *State) KgdbMode() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kgdbMode
}

func (s *State) SetKgdbMode(v bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.kgdbMode = v
	s.mu.Unlock()
}

// ArmSkipKgdbAttachStackRefresh skips the next kgdb post-stop stack MI query
// (used after target remote on slow serial mux).
func (s *State) ArmSkipKgdbAttachStackRefresh() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.skipKgdbAttachStackRefresh = true
	s.mu.Unlock()
}

// TakeSkipKgdbAttachStackRefresh reports and clears the attach skip flag.
func (s *State) TakeSkipKgdbAttachStackRefresh() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	v := s.skipKgdbAttachStackRefresh
	s.skipKgdbAttachStackRefresh = false
	s.mu.Unlock()
	return v
}

// SuppressGdbConsole is true when the GDB widget should not paint DisplayLines.
func (s *State) SuppressGdbConsole() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.gdbListenPrint {
		return false
	}
	owner := platform.PTYOwnerNone
	if s.app != nil {
		owner = s.app.PTYOwner()
	}
	if owner == platform.PTYOwnerApp || owner == platform.PTYOwnerMCP {
		return true
	}
	return s.gdbConsoleSilent
}

func (s *State) InferiorRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inferiorRunning
}

func (s *State) SetInferiorRunning(v bool) {
	s.mu.Lock()
	s.inferiorRunning = v
	s.mu.Unlock()
}

// NoteTransientStopSuppress arms one skip of Code/threads stop UI for the
// next *stopped from a break/clear interrupt that will auto-continue.
func (s *State) NoteTransientStopSuppress() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopUISuppress++
	s.mu.Unlock()
}

// ConsumeStopUISuppress returns true once per NoteTransientStopSuppress.
func (s *State) ConsumeStopUISuppress() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopUISuppress <= 0 {
		return false
	}
	s.stopUISuppress--
	return true
}

func (s *State) SourceFiles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.sourceFiles))
	copy(out, s.sourceFiles)
	return out
}

func (s *State) SetSourceFiles(files []string) {
	s.mu.Lock()
	s.sourceFiles = append([]string(nil), files...)
	s.mu.Unlock()
}

func (s *State) CurrentFile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentFile
}

func (s *State) CurrentLine() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentLine
}

func (s *State) SetCurrentLocation(file string, line int) {
	s.mu.Lock()
	s.currentFile = file
	s.currentLine = line
	s.mu.Unlock()
}

func (s *State) StopFile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopFile
}

func (s *State) StopLine() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopLine
}

func (s *State) SetStopLocation(file string, line int) {
	s.mu.Lock()
	s.stopFile = file
	s.stopLine = line
	s.mu.Unlock()
}

func (s *State) ClearStopLocation() {
	s.mu.Lock()
	s.stopFile = ""
	s.stopLine = 0
	s.mu.Unlock()
}

func (s *State) BreakColor() tcell.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.breakColor == tcell.ColorDefault {
		return platform.DefaultBreakColor
	}
	return s.breakColor
}

func (s *State) SetBreakColor(c tcell.Color) {
	s.mu.Lock()
	s.breakColor = c
	s.mu.Unlock()
}

func (s *State) BreakDisabledColor() tcell.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.breakDisabledColor == tcell.ColorDefault {
		return platform.DefaultBreakDisabledColor
	}
	return s.breakDisabledColor
}

func (s *State) SetBreakDisabledColor(c tcell.Color) {
	s.mu.Lock()
	s.breakDisabledColor = c
	s.mu.Unlock()
}

func (s *State) BreakCondColor() tcell.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.breakCondColor == tcell.ColorDefault {
		return platform.DefaultBreakCondColor
	}
	return s.breakCondColor
}

func (s *State) SetBreakCondColor(c tcell.Color) {
	s.mu.Lock()
	s.breakCondColor = c
	s.mu.Unlock()
}

func (s *State) PCColor() tcell.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pcColor == tcell.ColorDefault {
		return platform.DefaultPCColor
	}
	return s.pcColor
}

func (s *State) SetPCColor(c tcell.Color) {
	s.mu.Lock()
	s.pcColor = c
	s.mu.Unlock()
}

func (s *State) StackBreakColor() tcell.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stackBreakColor == tcell.ColorDefault {
		return platform.DefaultStackBreakColor
	}
	return s.stackBreakColor
}

func (s *State) SetStackBreakColor(c tcell.Color) {
	s.mu.Lock()
	s.stackBreakColor = c
	s.mu.Unlock()
}

// Mark / list-selection colors stay on platform.AppState; forwarded for widgets.
func (s *State) MarkColor() tcell.Color {
	if s.app != nil {
		return s.app.MarkColor()
	}
	return platform.DefaultMarkColor
}

func (s *State) MarkDimColor() tcell.Color {
	if s.app != nil {
		return s.app.MarkDimColor()
	}
	return platform.DefaultMarkDimColor
}

func (s *State) CodeSelColor() tcell.Color {
	if s.app != nil {
		return s.app.CodeSelColor()
	}
	return platform.DefaultCodeSelColor
}

func (s *State) MutedColor() tcell.Color {
	if s.app != nil {
		return s.app.MutedColor()
	}
	return platform.DefaultMutedColor
}

func (s *State) SearchColor() tcell.Color {
	if s.app != nil {
		return s.app.SearchColor()
	}
	return platform.DefaultSearchColor
}

func (s *State) SetMarkColor(c tcell.Color) {
	if s != nil && s.app != nil {
		s.app.SetMarkColor(c)
	}
}

func (s *State) SetMarkDimColor(c tcell.Color) {
	if s != nil && s.app != nil {
		s.app.SetMarkDimColor(c)
	}
}

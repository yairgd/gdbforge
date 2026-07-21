package widgets

import (
	"fmt"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
	"github.com/yairgd/gdbforge/internal/termui"
)

// GDBWidget is a ConsolePane wired to a GDB MI session (native REPL look).
// It owns the GDBClient lifetime: create via NewGDBWidget, start the UI
// bridge with Start, tear down with Close.
type GDBWidget struct {
	console       *termui.ConsolePane
	client        *gdb.GDBClient
	outputChan    <-chan core.PtyOutputMsg
	cancelSub     func()
	gdbInputState gdb.GdbInputState
	appState      *platform.AppState
	onStopped     func(*gdb.MiStopMsg)
	onBreakpoints func()
	onRunning     func()
	onFrameSync   func()
	pendingFrameSync bool
	// MI suppresses CLI "Quit anyway?" — track inferior and mirror that prompt.
	inferiorPID  string
	inferiorAlive bool
	quitConfirm  bool
}

func NewGDBWidget(gdbPath, prog string, args ...string) (*GDBWidget, error) {
	client, err := gdb.NewGDBClient(gdbPath, prog, args...)
	if err != nil {
		return nil, err
	}

	console := termui.NewConsolePane("GDB")
	// No standing Prompt: Draw must not invent "(gdb)" while waiting.
	// The live host is attached only when MI emits the (gdb) marker.
	console.Prompt = ""
	console.PromptStyle = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	console.LineStyle = gdbLineStyle

	ch, cancel := client.Subscribe()
	w := &GDBWidget{
		console:       console,
		client:        client,
		outputChan:    ch,
		cancelSub:     cancel,
		gdbInputState: *gdb.NewGdbInputState(),
	}

	console.OnSubmit = w.onSubmit
	console.OnInterrupt = w.onInterrupt
	console.OnEOF = w.onEOF
	return w, nil
}

func gdbLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if strings.HasPrefix(line, ">>>") {
		return st.Foreground(tcell.ColorTeal).Bold(true)
	}
	if strings.Contains(line, "❌️ Quit") {
		return st.Foreground(tcell.ColorRed).Bold(true)
	}
	if strings.HasPrefix(line, "(gdb)") {
		return st.Foreground(tcell.ColorYellow)
	}
	return st
}

// SetAppState wires global app state (PTY owner tracking, etc.).
func (m *GDBWidget) SetAppState(s *platform.AppState) {
	m.appState = s
}

// SetOnStopped registers a callback invoked on *stopped (breakpoint / step).
func (m *GDBWidget) SetOnStopped(fn func(*gdb.MiStopMsg)) {
	m.onStopped = fn
}

// SetOnBreakpointsChanged registers a callback for =breakpoint-created/deleted/modified.
func (m *GDBWidget) SetOnBreakpointsChanged(fn func()) {
	m.onBreakpoints = fn
}

// SetOnFrameSync registers a callback after CLI/MI stack navigation
// (frame / f / up / down / -stack-select-frame) completes (^done).
func (m *GDBWidget) SetOnFrameSync(fn func()) {
	m.onFrameSync = fn
}

// SetOnRunning registers a callback for ^running (inferior resumed).
func (m *GDBWidget) SetOnRunning(fn func()) {
	m.onRunning = fn
}

// Session exposes the owned debugger for external APIs (e.g. MCP).
// Callers may Send and Subscribe; they must not Close while the widget owns it.
func (m *GDBWidget) Session() core.Session {
	return m.client
}

// InferiorTTY returns the dedicated program I/O PTY, or nil.
func (m *GDBWidget) InferiorTTY() *ptyx.TTY {
	if m == nil || m.client == nil {
		return nil
	}
	return m.client.InferiorTTY()
}

func (m *GDBWidget) Start(screen tcell.Screen) {
	if screen == nil || m.outputChan == nil {
		return
	}
	ch := m.outputChan
	// Coalesce PTY chunks off the UI thread so a free-running inferior's
	// stdout does not PostEvent (and full-redraw) for every tiny read.
	// Painting still happens only on the tcell event loop.
	go coalesceGdbOutput(ch, func(msg core.GdbOutputMsg) {
		_ = screen.PostEvent(tcell.NewEventInterrupt(msg))
	}, func() {
		_ = screen.PostEvent(tcell.NewEventInterrupt("gdb-exit"))
	})
}

const (
	gdbOutputFlushInterval = 16 * time.Millisecond
	gdbOutputFlushMaxBytes = 64 * 1024
)

func coalesceGdbOutput(ch <-chan core.PtyOutputMsg, post func(core.GdbOutputMsg), onExit func()) {
	var pending strings.Builder
	var flushTimer *time.Timer
	var flushC <-chan time.Time

	disarm := func() {
		if flushTimer == nil {
			return
		}
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushTimer = nil
		flushC = nil
	}
	flush := func() {
		disarm()
		if pending.Len() == 0 {
			return
		}
		data := pending.String()
		pending.Reset()
		post(core.GdbOutputMsg{Data: data})
	}
	arm := func() {
		if flushTimer != nil {
			return
		}
		flushTimer = time.NewTimer(gdbOutputFlushInterval)
		flushC = flushTimer.C
	}

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				flush()
				if onExit != nil {
					onExit()
				}
				return
			}
			if msg.Err != nil {
				flush()
				post(core.GdbOutputMsg{Err: msg.Err})
				continue
			}
			if msg.Data == "" {
				continue
			}
			pending.WriteString(msg.Data)
			if pending.Len() >= gdbOutputFlushMaxBytes {
				flush()
			} else {
				arm()
			}
		case <-flushC:
			flush()
		}
	}
}

func (m *GDBWidget) Close() {
	if m.cancelSub != nil {
		m.cancelSub()
		m.cancelSub = nil
	}
	if m.client != nil {
		m.client.Close()
		m.client = nil
	}
}

func (m *GDBWidget) SetClipboard(io termui.ClipboardIO) {
	m.console.SetClipboard(io)
}

func (m *GDBWidget) AppendLines(lines []string) {
	if len(lines) == 0 {
		return
	}
	m.console.AppendLines(lines)
	m.console.FollowTailAndScroll()
}

// AppendTargetText paints raw inferior stdout into the GDB console (legacy
// shared-terminal mode when :set gdbtargetprint is on).
func (m *GDBWidget) AppendTargetText(text string) {
	if text == "" {
		return
	}
	m.console.AppendText(text)
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) Clear() {
	m.console.Clear()
}

// InputText returns the current GDB input-line contents.
func (m *GDBWidget) InputText() string {
	if m == nil || m.console == nil || m.console.Input() == nil {
		return ""
	}
	return m.console.Input().Text()
}

// ApplyCompletion replaces the GDB input line with name (full-line MI -complete match).
func (m *GDBWidget) ApplyCompletion(name string) {
	if m == nil || m.console == nil || m.console.Input() == nil || name == "" {
		return
	}
	m.console.Input().SetText(name)
	m.console.FollowTailAndScroll()
}

// Completer returns a commands.Completer backed by MI -complete on this session.
func (m *GDBWidget) Completer() commands.Completer {
	return func(prefix string) []string {
		if m == nil {
			return nil
		}
		return gdb.CompleteNames(m.Session(), m.appState, prefix)
	}
}

func (m *GDBWidget) SetFocused(focused bool) {
	m.console.SetFocused(focused)
}

func (m *GDBWidget) Draw(c termui.Canvas) {
	m.console.Draw(c)
}

func (m *GDBWidget) DrawStatusLine(c termui.Canvas, active bool) {
	m.console.DrawStatusLine(c, active)
}

func (m *GDBWidget) onSubmit(raw string) {
	if m.client == nil {
		return
	}

	// Finish a pending quit confirm (MI never asks; we mirror CLI wording).
	if m.quitConfirm {
		m.finishQuitConfirm(raw)
		return
	}

	// Empty Enter at (gdb) repeats the previous command (CLI GDB behavior).
	cmd := raw
	if cmd == "" {
		cmd = m.console.Input().LastHistory()
	}

	if isQuitCmd(cmd) && m.inferiorAlive {
		m.beginQuitConfirm(cmd)
		return
	}

	if isStackNavCmd(cmd) {
		m.pendingFrameSync = true
	}
	send := func() {
		_ = m.client.Send(cmd)
	}
	if cmd != "" {
		m.console.Input().PushHistory(cmd)
		m.console.EchoSubmit(cmd)
	}
	if m.appState != nil {
		m.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
	m.console.Input().Clear()
	m.console.FollowTailAndScroll()
}

// beginQuitConfirm shows the standard CLI quit prompt. MI auto-exits without
// asking, so we gate Send("q") until the user answers y/n.
func (m *GDBWidget) beginQuitConfirm(cmd string) {
	m.quitConfirm = true
	if cmd != "" {
		m.console.Input().PushHistory(cmd)
		m.console.EchoSubmit(cmd)
	}
	pid := m.inferiorPID
	if pid == "" {
		pid = "?"
	}
	m.console.AppendLines([]string{
		"A debugging session is active.",
		"",
		fmt.Sprintf("\tInferior 1 [process %s] will be killed.", pid),
		"",
	})
	if buf := m.console.Buffer(); buf != nil {
		buf.AppendLine("Quit anyway? (y or n) ")
	}
	m.console.SetLivePrompt(true)
	m.console.Input().Clear()
	m.console.FollowTailAndScroll()
}

func (m *GDBWidget) finishQuitConfirm(raw string) {
	ans := strings.TrimSpace(strings.ToLower(raw))
	// Bare Enter = default [n], matching CLI.
	yes := ans == "y" || ans == "yes"
	no := ans == "" || ans == "n" || ans == "no"
	if !yes && !no {
		// Re-prompt on garbage (same as keeping the question open).
		if buf := m.console.Buffer(); buf != nil {
			buf.AppendLine("Please answer y or n.")
			buf.AppendLine("Quit anyway? (y or n) ")
		}
		m.console.SetLivePrompt(true)
		m.console.Input().Clear()
		m.console.FollowTailAndScroll()
		return
	}

	display := ans
	if display == "" {
		display = "n"
	}
	m.console.EchoSubmit(display)
	m.console.Input().Clear()
	m.quitConfirm = false

	if !yes {
		// Solicit a real MI (gdb) prompt — do not invent one locally.
		if m.client != nil {
			send := func() { _ = m.client.Send("") }
			if m.appState != nil {
				m.appState.WithPTYOwner(platform.PTYOwnerUI, send)
			} else {
				send()
			}
		}
		m.console.FollowTailAndScroll()
		return
	}

	if m.client == nil {
		return
	}
	send := func() { _ = m.client.Send("quit") }
	if m.appState != nil {
		m.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
	m.console.FollowTailAndScroll()
}

func isQuitCmd(cmd string) bool {
	fields := strings.Fields(strings.TrimSpace(cmd))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "q", "quit":
		return true
	default:
		return false
	}
}

// isStackNavCmd reports CLI/MI commands that change the selected stack frame
// without a *stopped event.
func isStackNavCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "frame", "f", "up", "down", "select-frame":
		return true
	}
	return strings.HasPrefix(fields[0], "-stack-select-frame")
}

func (m *GDBWidget) onInterrupt() {
	if m.client == nil {
		return
	}
	// Clear partial input; Quit text comes from MI &"Quit" / &"❌️ Quit", not UI.
	m.console.Input().Clear()
	send := func() { _ = m.client.SendRaw("\x03") }
	if m.appState != nil {
		m.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
}

func (m *GDBWidget) onEOF() {
	if m.client == nil {
		return
	}
	// Same path as typing q — confirm when an inferior is alive.
	if m.inferiorAlive && !m.quitConfirm {
		m.beginQuitConfirm("q")
		return
	}
	send := func() { _ = m.client.Send("q") }
	if m.appState != nil {
		m.appState.WithPTYOwner(platform.PTYOwnerUI, send)
	} else {
		send()
	}
}

func (m *GDBWidget) handleStop(stop *gdb.MiStopMsg) {
	if stop == nil {
		return
	}
	switch stop.Reason {
	case "exited-normally", "exited", "exited-signalled":
		// Inferior is gone — no threads/stack refresh.
		return
	default:
		// breakpoint-hit, end-stepping-range, signal-received (Ctrl-C), …
		if m.onStopped != nil {
			m.onStopped(stop)
		}
	}
}

// silentOwner is true when the GDB console should not paint PTY DisplayLines:
// App/MCP (listener) traffic, sticky after those writes. UI submit clears
// sticky silence. Default gdblistenprint paints listener replies; :set nogdblistenprint hides them.
func (m *GDBWidget) silentOwner() bool {
	if m.appState == nil {
		return false
	}
	return m.appState.SuppressGdbConsole()
}

func (m *GDBWidget) applyMiUpdate(upd gdb.MiUpdate) {
	if upd.InferiorPID != "" {
		m.inferiorPID = upd.InferiorPID
		m.inferiorAlive = true
	}
	if upd.InferiorExited {
		m.inferiorPID = ""
		m.inferiorAlive = false
		m.quitConfirm = false
	}

	silent := m.silentOwner()
	if !silent {
		lines := upd.DisplayLines
		if m.appState != nil && m.appState.GdbTargetPrint() {
			lines = append(lines, upd.TargetLines...)
		}
		var rest []string
		painted := false
		for _, line := range lines {
			// Attach Ctrl-C Quit log to the live (gdb) prompt line.
			if gdb.IsCtrlCQuitLog(line) {
				m.console.EchoSubmit(line)
				painted = true
				continue
			}
			rest = append(rest, line)
		}
		if len(rest) > 0 {
			m.console.AppendLines(rest)
			m.stripTrailingGdbPrompt()
			painted = true
		}
		// Materialize the MI (gdb) marker as the live host — never invent it earlier.
		if upd.PromptReady && !m.quitConfirm {
			m.attachGdbPrompt()
		}
		if painted || (upd.PromptReady && !m.quitConfirm) {
			m.console.FollowTailAndScroll()
		}
	}
	if upd.Stopped != nil {
		if m.appState != nil {
			m.appState.SetInferiorRunning(false)
		}
		if reason := upd.Stopped.Reason; reason == "exited-normally" || reason == "exited" || reason == "exited-signalled" {
			m.inferiorAlive = false
			m.inferiorPID = ""
		}
		m.handleStop(upd.Stopped)
	}
	if m.pendingFrameSync {
		if upd.State == gdb.Error {
			m.pendingFrameSync = false
		} else if upd.PromptReady {
			// Wait for (gdb) so ^done + console frame text have arrived.
			// Do not key off State==Done: zero-value MiUpdate is Done.
			m.pendingFrameSync = false
			if m.onFrameSync != nil {
				m.onFrameSync()
			}
		}
	}
	if upd.State == gdb.Running {
		if m.appState != nil {
			m.appState.SetInferiorRunning(true)
		}
		if m.onRunning != nil {
			m.onRunning()
		}
	}
	if upd.BreakpointsChanged && m.onBreakpoints != nil {
		m.onBreakpoints()
	}
}

// attachGdbPrompt paints the MI (gdb) prompt record as the live input host.
func (m *GDBWidget) attachGdbPrompt() {
	const host = "(gdb) "
	if buf := m.console.Buffer(); buf != nil {
		n := buf.NumLines()
		if n > 0 && buf.Line(n-1) == host {
			m.console.SetLivePrompt(true)
			return
		}
		buf.AppendLine(host)
	}
	m.console.SetLivePrompt(true)
}

// stripTrailingGdbPrompt drops a trailing bare (gdb) host before new output.
func (m *GDBWidget) stripTrailingGdbPrompt() {
	buf := m.console.Buffer()
	if buf == nil {
		return
	}
	for buf.NumLines() > 0 {
		last := strings.TrimSpace(buf.Line(buf.NumLines() - 1))
		if last != "" && last != "(gdb)" {
			return
		}
		buf.RemoveLine(buf.NumLines() - 1)
		m.console.SetLivePrompt(false)
		if last == "(gdb)" {
			return
		}
	}
}

func (m *GDBWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventInterrupt:
		if data, ok := e.Data().(core.GdbOutputMsg); ok && data.Data != "" {
			m.applyMiUpdate(m.gdbInputState.PushRaw(data.Data))
		}
	default:
		m.console.HandleEvent(ev)
	}
}

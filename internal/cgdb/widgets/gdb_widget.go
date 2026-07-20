package widgets

import (
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/platform"
	"github.com/yairgd/cgdb-go/internal/termui"
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
}

func NewGDBWidget(gdbPath, prog string, args ...string) (*GDBWidget, error) {
	client, err := gdb.NewGDBClient(gdbPath, prog, args...)
	if err != nil {
		return nil, err
	}

	console := termui.NewConsolePane("GDB")
	console.Prompt = "(gdb) "
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

func (m *GDBWidget) Clear() {
	m.console.Clear()
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
	cmd := raw
	if cmd == "" {
		cmd = m.console.Input().LastHistory()
	}
	if isStackNavCmd(cmd) {
		m.pendingFrameSync = true
	}
	send := func() {
		if cmd != "" {
			_ = m.client.Send(cmd)
		} else {
			_ = m.client.Send("")
		}
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
	silent := m.silentOwner()
	if !silent {
		if len(upd.DisplayLines) > 0 {
			m.console.AppendLines(upd.DisplayLines)
			m.console.StripTrailingBarePrompt()
		}
		if upd.PromptReady {
			m.console.EnsureLivePrompt()
		}
		if len(upd.DisplayLines) > 0 || upd.PromptReady {
			m.console.FollowTailAndScroll()
		}
	}
	if upd.Stopped != nil {
		if m.appState != nil {
			m.appState.SetInferiorRunning(false)
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

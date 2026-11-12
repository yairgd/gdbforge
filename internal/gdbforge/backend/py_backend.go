package backend

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugger"
	"github.com/yairgd/gdbforge/internal/pydbg"
	"github.com/yairgd/termforge/platform"
	"github.com/yairgd/termforge/ptyx"
)

// PYBackend is the Python (debugpy over DAP) implementation of Backend.
// Machine control goes straight to pydbg.Client; the console PTY only carries
// text for the human. Client events are queued and turned into a
// debugger.ConsoleUpdate when console text arrives (PushConsoleOutput).
type PYBackend struct {
	Client *pydbg.Client

	// resuming is set from a resume request until the target stops or exits.
	resuming atomic.Bool

	mu     sync.Mutex
	events []pydbg.Event
}

var _ Backend = (*PYBackend)(nil)

func NewPY(client *pydbg.Client) *PYBackend {
	b := &PYBackend{Client: client}
	if client != nil {
		client.SetOnEvent(b.enqueue)
	}
	return b
}

// enqueue is the Client event handler (called from the DAP goroutine).
func (b *PYBackend) enqueue(ev pydbg.Event) {
	switch ev.Kind {
	case pydbg.Stopped, pydbg.Exited:
		b.resuming.Store(false)
	}
	b.mu.Lock()
	b.events = append(b.events, ev)
	b.mu.Unlock()
}

func (b *PYBackend) client() *pydbg.Client {
	if b == nil {
		return nil
	}
	return b.Client
}

func (b *PYBackend) Kind() Kind            { return PY }
func (b *PYBackend) Session() ptyx.Session { return b.client() }
func (b *PYBackend) Close() {
	if c := b.client(); c != nil {
		c.Close()
	}
}

func (b *PYBackend) TakeStartupOutput() string {
	if c := b.client(); c != nil {
		return c.TakeStartupOutput()
	}
	return ""
}
func (b *PYBackend) InferiorTTY() *ptyx.TTY {
	if c := b.client(); c != nil {
		return c.InferiorTTY()
	}
	return nil
}

// ConfigureInferiorTTY: pydbg wires the inferior PTY when it starts the program.
func (b *PYBackend) ConfigureInferiorTTY() error { return nil }

// SetInferiorTTYPath only accepts the internal tty; an external one needs a restart.
func (b *PYBackend) SetInferiorTTYPath(path string) error {
	if p := strings.TrimSpace(path); p == "" || p == "internal" {
		return nil
	}
	return ErrNotSupported
}
func (b *PYBackend) PromptToken() string        { return pydbg.PromptToken }
func (b *PYBackend) PaintTargetInConsole() bool { return false }

func (b *PYBackend) SupportsAssembly() bool        { return false }
func (b *PYBackend) SupportsSourceFileList() bool  { return b.client() != nil }
func (b *PYBackend) SupportsLiveInferiorTTY() bool { return false }

// BreakRefreshImmediate: table changes made through Client are synchronous, so
// the gutter can refresh right away (console-typed ones also arrive as events).
func (b *PYBackend) BreakRefreshImmediate() bool { return true }

func (b *PYBackend) MapExec(cmd string) (string, bool) {
	send := strings.TrimSpace(cmd)
	switch send {
	case "run", "start":
		send = "restart"
	}
	return send, IsRunCmd(send) || IsRunCmd(cmd)
}
func (b *PYBackend) MapBreak(cmd string) string      { return cmd }
func (b *PYBackend) DefaultBreakMain() string        { return "" }
func (b *PYBackend) InfoRegistersCmd() string        { return "" }
func (b *PYBackend) InfoThreadsCmd() string          { return "threads" }
func (b *PYBackend) SelectFrameCmd(level int) string { return "frame " + strconv.Itoa(level) }
func (b *PYBackend) SelectThreadCmd(id string) string {
	return "thread " + id
}

// Complete asks debugpy for Python completions in the selected frame; the
// candidates replace the identifier at the end of text.
func (b *PYBackend) Complete(_ ptyx.Session, _ *platform.AppState, text string) gdb.CompleteResult {
	c := b.client()
	if c == nil {
		return gdb.CompleteResult{}
	}
	targets, err := c.Complete(text)
	if err != nil || len(targets) == 0 {
		return gdb.CompleteResult{}
	}
	base := text[:strings.LastIndexFunc(text, func(r rune) bool {
		return !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127)
	})+1]
	full := make([]string, len(targets))
	for i, t := range targets {
		full[i] = base + t
	}
	return gdb.CompleteResult{Completion: base + commonPrefix(targets), Matches: full}
}

func (b *PYBackend) EnrichLinespecMenu(_ string, menu []string, _ ptyx.Session, _ *platform.AppState) []string {
	return menu
}

func (b *PYBackend) Interrupt(inferiorRunning, _ bool) error {
	c := b.client()
	if c == nil || !c.Running() {
		return nil
	}
	return c.Pause()
}

func (b *PYBackend) TargetRunning() (bool, bool) {
	c := b.client()
	if c == nil {
		return false, false
	}
	return c.Running(), true
}

func (b *PYBackend) SuspendInferior() error { return ErrNotSupported }

func (b *PYBackend) SendLine(cmd string) error {
	if c := b.client(); c != nil {
		return c.Send(cmd)
	}
	return nil
}

// promptReady reports whether console text ends with the (py) prompt.
func promptReady(data string) bool {
	return strings.HasSuffix(strings.TrimRight(platform.StripANSI(data), " \t\r\n"), pydbg.PromptToken)
}

// PushConsoleOutput turns every queued Client event into one ConsoleUpdate.
// Events never repeat (they are drained) and never get lost (an event that
// arrives after its banner is returned by the next push).
func (b *PYBackend) PushConsoleOutput(data string) debugger.ConsoleUpdate {
	b.mu.Lock()
	evs := b.events
	b.events = nil
	b.mu.Unlock()
	u := debugger.ConsoleUpdate{PromptReady: promptReady(data)}
	for _, ev := range evs {
		switch ev.Kind {
		case pydbg.Stopped:
			if exitReason(ev.Reason) {
				u.InferiorExited, u.Stopped, u.State = true, nil, debugger.StateDone
				break
			}
			u.Stopped = &debugger.StopInfo{
				Reason:         ev.Reason,
				ThreadID:       ev.ThreadID,
				SourceLocation: debugger.SourceLocation{File: ev.File, Line: ev.Line, Func: ev.Func},
			}
			u.InferiorExited, u.State = false, debugger.StateDone
		case pydbg.Exited:
			u.InferiorExited, u.Stopped, u.State = true, nil, debugger.StateDone
		case pydbg.Running:
			// A later stop or exit in the same batch wins.
			u.Stopped, u.InferiorExited, u.State = nil, false, debugger.StateRunning
		case pydbg.BreaksChanged:
			u.BreakpointsChanged = true
		case pydbg.FrameSelected:
			u.FrameSelected = &debugger.FrameInfo{Level: ev.Level, Func: ev.Func, File: ev.File, Line: ev.Line}
		}
	}
	return u
}

func exitReason(r string) bool {
	switch r {
	case "exited-normally", "exited", "exited-signalled":
		return true
	}
	return false
}

func (b *PYBackend) Confirming() bool { return false }

func (b *PYBackend) InferiorTTYPath() string {
	if t := b.InferiorTTY(); t != nil {
		return t.SlaveName()
	}
	return ""
}

func (b *PYBackend) UsesExternalInferiorTTY() bool {
	t := b.InferiorTTY()
	return t != nil && t.IsExternal()
}

func (b *PYBackend) RequiresInferiorTTYRestart() bool { return true }

func (b *PYBackend) NavigationAsync() bool        { return false }
func (b *PYBackend) ConsoleEOFCommand() string    { return "quit" }
func (b *PYBackend) WireCLILineTap() bool         { return false }
func (b *PYBackend) DeferBreakpointRefresh() bool { return false }

// Evaluate runs a Python expression or statement in the selected frame.
func (b *PYBackend) Evaluate(_ context.Context, expr string) (string, error) {
	c := b.client()
	if c == nil {
		return "", ErrNotSupported
	}
	return c.Evaluate(expr)
}

func commonPrefix(ss []string) string {
	p := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
		}
	}
	return p
}

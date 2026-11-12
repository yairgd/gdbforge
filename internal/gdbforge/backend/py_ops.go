package backend

import (
	"context"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

// SendMappedBreak types a break/clear/... line on the (py) console.
func (b *PYBackend) SendMappedBreak(_ CommandEnv, cmd string) {
	if cmd != "" {
		_ = b.SendLine(b.MapBreak(cmd))
	}
}

func (b *PYBackend) InsertBreakpoint(_ CommandEnv, file string, line int) {
	if c := b.client(); c != nil {
		_, _ = c.SetBreak(file, line)
	}
}

func (b *PYBackend) ClearBreakpointAt(_ CommandEnv, file string, line int, number int) {
	if c := b.client(); c != nil {
		_ = c.ClearBreak(file, line, number)
	}
}

// ClearBreakpointAddr, InsertBreakpointAddr: Python has no address breakpoints.
func (b *PYBackend) ClearBreakpointAddr(CommandEnv, string, int) {}
func (b *PYBackend) InsertBreakpointAddr(CommandEnv, string)     {}

func (b *PYBackend) DisableBreakpoint(_ CommandEnv, number int) {
	if c := b.client(); c != nil {
		_ = c.EnableBreak(number, false)
	}
}

func (b *PYBackend) SetBreakpointCondition(_ CommandEnv, number int, cond string) {
	if c := b.client(); c != nil {
		_ = c.SetCondition(number, cond)
	}
}

// InsertDefaultBreakMain: a script starts stopped on its first line.
func (b *PYBackend) InsertDefaultBreakMain(CommandEnv) {}

// SelectFrame moves the selected frame; the UI follows from the FrameSelected event.
func (b *PYBackend) SelectFrame(_ CommandEnv, level int, _ NavigationOpts) {
	if c := b.client(); c != nil {
		_ = c.SelectFrame(level)
	}
}

func (b *PYBackend) SelectThread(_ CommandEnv, id string, _ NavigationOpts) {
	if c := b.client(); c != nil {
		_ = c.SelectThread(id)
	}
}

func (b *PYBackend) Exec(env CommandEnv, cmd string)   { b.exec(env, cmd) }
func (b *PYBackend) ExecUI(env CommandEnv, cmd string) { b.exec(env, cmd) }

// exec runs a run-control command through the Client (async; the outcome
// arrives as events). Other text goes to the console for the dispatcher.
func (b *PYBackend) exec(env CommandEnv, cmd string) {
	c := b.client()
	if c == nil {
		return
	}
	send, marksRunning := b.MapExec(cmd)
	var fn func() error
	switch strings.Fields(send + " x")[0] {
	case "c", "continue":
		fn = c.Continue
	case "n", "next":
		fn = c.Next
	case "s", "step":
		fn = c.StepIn
	case "finish", "stepout":
		fn = c.StepOut
	case "restart":
		fn = c.Restart
	default:
		_ = c.Send(send)
		return
	}
	// A second resume while one is in flight is ignored.
	if !b.resuming.CompareAndSwap(false, true) {
		return
	}
	if marksRunning {
		if rs, ok := env.Inferior.(interface{ SetInferiorRunning(bool) }); ok {
			rs.SetInferiorRunning(true)
		}
	}
	go func() {
		if err := fn(); err != nil {
			b.resuming.Store(false)
			if rs, ok := env.Inferior.(interface{ SetInferiorRunning(bool) }); ok && marksRunning {
				rs.SetInferiorRunning(false)
			}
		}
	}()
}

func (b *PYBackend) FetchBreakpoints(_ context.Context, _ Querier) ([]models.BreakInfo, bool) {
	c := b.client()
	if c == nil {
		return nil, false
	}
	return c.Breakpoints(), true
}

func (b *PYBackend) RefreshThreadsAndStack(_ context.Context, _ Querier, log LogFn) ([]models.ThreadInfo, []models.StackFrame, bool, bool) {
	c := b.client()
	if c == nil || c.Running() {
		return nil, nil, false, false
	}
	th, err := c.Threads()
	threadsOK := err == nil
	if err != nil && log != nil {
		log("threads", err.Error())
	}
	fr, err := c.Stack()
	stackOK := err == nil && len(fr) > 0
	if err != nil && log != nil {
		log("callstack", err.Error())
	}
	return th, fr, threadsOK, stackOK
}

// CurrentFrame is the top frame: Stack lists every frame and has no selected-frame marker.
func (b *PYBackend) CurrentFrame(ctx context.Context, q Querier) (models.StackFrame, bool) {
	fr, ok := b.FetchStackList(ctx, q, false)
	if !ok {
		return models.StackFrame{}, false
	}
	return fr[0], true
}

func (b *PYBackend) FetchStackList(_ context.Context, _ Querier, _ bool) ([]models.StackFrame, bool) {
	c := b.client()
	if c == nil || c.Running() {
		return nil, false
	}
	fr, err := c.Stack()
	return fr, err == nil && len(fr) > 0
}

func (b *PYBackend) ListSourceFiles(_ context.Context, _ Querier) ([]string, bool) {
	c := b.client()
	if c == nil {
		return nil, false
	}
	files, err := c.SourceFiles()
	return files, err == nil && len(files) > 0
}

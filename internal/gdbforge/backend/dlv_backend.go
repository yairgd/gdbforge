package backend

import (
	"context"
	"fmt"
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// DLVBackend is the Delve CLI implementation of Backend.
type DLVBackend struct {
	Client *dlv.Client
	Input  *dlv.InputState
}

func NewDLV(client *dlv.Client) *DLVBackend {
	return &DLVBackend{
		Client: client,
		Input:  dlv.NewInputState(),
	}
}

func (b *DLVBackend) Kind() Kind            { return DLV }
func (b *DLVBackend) Session() core.Session { return b.client() }
func (b *DLVBackend) Close() {
	if c := b.client(); c != nil {
		c.Close()
	}
}
func (b *DLVBackend) client() *dlv.Client {
	if b == nil {
		return nil
	}
	return b.Client
}

// ReplaceClient swaps the live Delve session (e.g. after --tty restart).
func (b *DLVBackend) ReplaceClient(client *dlv.Client) {
	if b == nil {
		return
	}
	b.Client = client
	b.Input = dlv.NewInputState()
}

func (b *DLVBackend) TakeStartupOutput() string {
	if c := b.client(); c != nil {
		return c.TakeStartupOutput()
	}
	return ""
}
func (b *DLVBackend) InferiorTTY() *ptyx.TTY {
	if c := b.client(); c != nil {
		return c.InferiorTTY()
	}
	return nil
}
func (b *DLVBackend) ConfigureInferiorTTY() error {
	if c := b.client(); c != nil {
		return c.ConfigureInferiorTTY()
	}
	return nil
}
func (b *DLVBackend) PromptToken() string        { return dlv.PromptToken }
func (b *DLVBackend) PaintTargetInConsole() bool { return true }

func (b *DLVBackend) SupportsAssembly() bool        { return false }
func (b *DLVBackend) SupportsSourceFileList() bool  { return false }
func (b *DLVBackend) SupportsLiveInferiorTTY() bool { return false }
func (b *DLVBackend) BreakRefreshImmediate() bool   { return false }

func (b *DLVBackend) MapExec(cmd string) (string, bool) {
	send := strings.TrimSpace(cmd)
	switch send {
	case "finish":
		send = "stepout"
	case "run", "start":
		send = "restart"
	}
	return send, IsRunCmd(send) || IsRunCmd(cmd)
}
func (b *DLVBackend) MapBreak(cmd string) string { return dlv.MapBreakCmd(cmd) }
func (b *DLVBackend) DefaultBreakMain() string   { return "break main.main" }
func (b *DLVBackend) InfoRegistersCmd() string   { return "regs" }
func (b *DLVBackend) InfoThreadsCmd() string     { return "threads" }
func (b *DLVBackend) SelectFrameCmd(level int) string {
	return fmt.Sprintf("frame %d", level)
}
func (b *DLVBackend) SelectThreadCmd(id string) string {
	return "goroutine " + id
}

func (b *DLVBackend) FetchBreakpoints(ctx context.Context, q Querier) ([]models.BreakInfo, bool) {
	if q == nil {
		return nil, false
	}
	raw, err := q.Query(ctx, "breakpoints")
	if err != nil {
		return nil, false
	}
	items := dlv.ParseBreakpoints(raw)
	low := strings.ToLower(raw)
	if len(items) == 0 {
		if strings.Contains(low, "no breakpoints") {
			return items, true
		}
		if strings.Contains(low, "breakpoint") {
			return nil, false
		}
		return nil, false
	}
	return items, true
}

func (b *DLVBackend) RefreshThreadsAndStack(ctx context.Context, q Querier, log LogFn) ([]models.ThreadInfo, []models.StackFrame, bool, bool) {
	return ThreadsAndStack(ctx, DLV, q, log)
}

func (b *DLVBackend) Complete(sess core.Session, state *platform.AppState, text string) gdb.CompleteResult {
	return dlv.Complete(sess, state, text)
}

func (b *DLVBackend) EnrichLinespecMenu(text string, menu []string, sess core.Session, state *platform.AppState) []string {
	_ = text
	_ = sess
	_ = state
	return menu
}

func (b *DLVBackend) Interrupt(inferiorRunning, confirming bool) error {
	c := b.client()
	if c == nil {
		return nil
	}
	if confirming {
		return c.Send("n")
	}
	if !inferiorRunning {
		return nil
	}
	return c.Interrupt()
}

func (b *DLVBackend) SuspendInferior() error { return ErrNotSupported }

func (b *DLVBackend) SendLine(cmd string) error {
	if c := b.client(); c != nil {
		return c.Send(cmd)
	}
	return nil
}

func (b *DLVBackend) PushConsoleOutput(data string) ConsoleEvent {
	if b.Input == nil {
		return ConsoleEvent{Kind: DLV}
	}
	u := b.Input.PushRaw(data)
	return ConsoleEvent{Kind: DLV, DLV: &u}
}

// AsDLVUpdate extracts the Delve update from a ConsoleEvent.
func AsDLVUpdate(ev ConsoleEvent) *dlv.Update {
	if u, ok := ev.DLV.(*dlv.Update); ok {
		return u
	}
	return nil
}

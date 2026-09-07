package backend

import (
	"fmt"
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/debugger"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// DLVBackend is the Delve CLI + rpc2 implementation of Backend.
type DLVBackend struct {
	Client  *dlv.Client
	Input   *dlv.InputState
	Confirm dlv.ConfirmGate
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
	b.Confirm.Clear()
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

func (b *DLVBackend) SetInferiorTTYPath(path string) error {
	if c := b.client(); c != nil {
		return c.SetInferiorTTYPath(path)
	}
	return ErrNotSupported
}
func (b *DLVBackend) PromptToken() string        { return dlv.PromptToken }
func (b *DLVBackend) PaintTargetInConsole() bool { return true }

func (b *DLVBackend) SupportsAssembly() bool        { return false }
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
		switch b.Confirm.Kind() {
		case dlv.ConfirmPauseQuit:
			return c.Send("p")
		default:
			return c.Send("n")
		}
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

func (b *DLVBackend) PushConsoleOutput(data string) debugger.ConsoleUpdate {
	if b.Input == nil {
		return debugger.ConsoleUpdate{}
	}
	u := b.Input.PushRaw(data)
	b.Confirm.Observe(u)
	return debugger.FromDLVUpdate(u)
}

func (b *DLVBackend) Confirming() bool { return b.Confirm.Confirming() }

func (b *DLVBackend) InferiorTTYPath() string {
	if c := b.client(); c != nil {
		return c.InferiorTTYPath()
	}
	return ""
}

func (b *DLVBackend) UsesExternalInferiorTTY() bool {
	if c := b.client(); c != nil {
		return c.UsesExternalInferiorTTY()
	}
	return false
}

func (b *DLVBackend) RequiresInferiorTTYRestart() bool { return true }

package backend

import (
	"context"
	"fmt"
	"strings"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// GDBBackend is the MI/GDB implementation of Backend.
type GDBBackend struct {
	Client *gdb.GDBClient
	Input  *gdb.GdbInputState
}

func NewGDB(client *gdb.GDBClient) *GDBBackend {
	return &GDBBackend{
		Client: client,
		Input:  gdb.NewGdbInputState(),
	}
}

func (b *GDBBackend) Kind() Kind              { return GDB }
func (b *GDBBackend) Session() core.Session   { return b.client() }
func (b *GDBBackend) Close() {
	if c := b.client(); c != nil {
		c.Close()
	}
}
func (b *GDBBackend) client() *gdb.GDBClient {
	if b == nil {
		return nil
	}
	return b.Client
}

func (b *GDBBackend) TakeStartupOutput() string {
	if c := b.client(); c != nil {
		return c.TakeStartupOutput()
	}
	return ""
}
func (b *GDBBackend) InferiorTTY() *ptyx.TTY {
	if c := b.client(); c != nil {
		return c.InferiorTTY()
	}
	return nil
}
func (b *GDBBackend) ConfigureInferiorTTY() error {
	if c := b.client(); c != nil {
		return c.ConfigureInferiorTTY()
	}
	return nil
}
func (b *GDBBackend) PromptToken() string       { return gdb.MIPromptToken }
func (b *GDBBackend) PaintTargetInConsole() bool { return false }

func (b *GDBBackend) SupportsAssembly() bool         { return true }
func (b *GDBBackend) SupportsSourceFileList() bool   { return true }
func (b *GDBBackend) SupportsLiveInferiorTTY() bool  { return true }
func (b *GDBBackend) BreakRefreshImmediate() bool    { return true }

func (b *GDBBackend) MapExec(cmd string) (string, bool) {
	send := gdb.CLIExecToMI(cmd)
	return send, IsRunCmd(cmd)
}
func (b *GDBBackend) MapBreak(cmd string) string { return cmd }
func (b *GDBBackend) DefaultBreakMain() string   { return "break main" }
func (b *GDBBackend) InfoRegistersCmd() string   { return "info registers" }
func (b *GDBBackend) InfoThreadsCmd() string     { return "info threads" }
func (b *GDBBackend) SelectFrameCmd(level int) string {
	return fmt.Sprintf("-stack-select-frame %d", level)
}
func (b *GDBBackend) SelectThreadCmd(id string) string {
	return "-thread-select " + id
}

func (b *GDBBackend) FetchBreakpoints(ctx context.Context, q Querier) ([]models.BreakInfo, bool) {
	if q == nil {
		return nil, false
	}
	raw, err := q.Query(ctx, "-break-list")
	if err != nil {
		return nil, false
	}
	if !strings.Contains(raw, "BreakpointTable") && !strings.Contains(raw, "bkpt={") {
		return nil, false
	}
	return parse.ParseBreakList(raw), true
}

func (b *GDBBackend) RefreshThreadsAndStack(ctx context.Context, q Querier, log LogFn) ([]models.ThreadInfo, []models.StackFrame, bool, bool) {
	return ThreadsAndStack(ctx, GDB, q, log)
}

func (b *GDBBackend) Complete(sess core.Session, state *platform.AppState, text string) gdb.CompleteResult {
	return gdb.Complete(sess, state, text)
}

func (b *GDBBackend) EnrichLinespecMenu(text string, menu []string, sess core.Session, state *platform.AppState) []string {
	if !gdb.CompletingLinespec(text) {
		return menu
	}
	if sigs := gdb.FunctionSignatures(sess, state); len(sigs) > 0 {
		return gdb.EnrichLinespecMenuNames(text, menu, sigs)
	}
	return menu
}

func (b *GDBBackend) Interrupt(inferiorRunning, confirming bool) error {
	_ = inferiorRunning
	_ = confirming
	if c := b.client(); c != nil {
		return c.Interrupt()
	}
	return nil
}

func (b *GDBBackend) SuspendInferior() error {
	if c := b.client(); c != nil {
		return c.SuspendInferior()
	}
	return nil
}

func (b *GDBBackend) SendLine(cmd string) error {
	if c := b.client(); c != nil {
		return c.Send(cmd)
	}
	return nil
}

func (b *GDBBackend) PushConsoleOutput(data string) ConsoleEvent {
	if b.Input == nil {
		return ConsoleEvent{Kind: GDB}
	}
	u := b.Input.PushRaw(data)
	return ConsoleEvent{Kind: GDB, GDB: &u}
}

// IsRunCmd reports continue/step/run-style commands (shared by GDB/DLV exec arming).
func IsRunCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	switch strings.Fields(cmd)[0] {
	case "c", "continue", "n", "next", "s", "step", "stepout", "finish", "nexti", "ni", "stepi", "si", "restart", "run", "start":
		return true
	default:
		return false
	}
}

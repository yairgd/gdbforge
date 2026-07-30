package backend

import (
	"context"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/ptyx"
)

// Backend is the GDB vs Delve policy surface. DebuggerApp wires UI to Session
// and calls these methods instead of scattering isDLV() branches.
type Backend interface {
	Kind() Kind
	Session() core.Session
	Close()

	TakeStartupOutput() string
	InferiorTTY() *ptyx.TTY
	ConfigureInferiorTTY() error
	PromptToken() string
	// SetGdbTargetPrint is true when program stdout should also paint in the debugger console (Delve).
	PaintTargetInConsole() bool

	SupportsAssembly() bool
	SupportsSourceFileList() bool
	SupportsLiveInferiorTTY() bool

	MapExec(cmd string) (sendCmd string, marksRunning bool)
	MapBreak(cmd string) string
	DefaultBreakMain() string
	InfoRegistersCmd() string
	InfoThreadsCmd() string
	SelectFrameCmd(level int) string
	SelectThreadCmd(id string) string

	// BreakRefreshImmediate is true when sendBreakpointCmd should call onBreakpointsChanged now (GDB).
	BreakRefreshImmediate() bool

	FetchBreakpoints(ctx context.Context, q Querier) ([]models.BreakInfo, bool)
	RefreshThreadsAndStack(ctx context.Context, q Querier, log LogFn) (threads []models.ThreadInfo, frames []models.StackFrame, threadsOK, stackOK bool)

	Complete(sess core.Session, state *platform.AppState, text string) gdb.CompleteResult
	EnrichLinespecMenu(text string, menu []string, sess core.Session, state *platform.AppState) []string

	// Interrupt sends SIGINT / cancel-confirm according to backend rules.
	Interrupt(inferiorRunning, confirming bool) error
	// SuspendInferior is GDB-only (SIGTSTP); DLV returns ErrNotSupported.
	SuspendInferior() error
	// SendLine writes a console line (UI owner must wrap if needed).
	SendLine(cmd string) error

	// PushConsoleOutput feeds PTY text into the backend input parser.
	PushConsoleOutput(data string) ConsoleEvent
}

// ConsoleEvent is a unified paint signal from PushConsoleOutput.
type ConsoleEvent struct {
	Kind Kind
	GDB  *gdb.MiUpdate
	DLV  any // *dlv.Update — typed in dlv_backend to avoid cycles in docs; use AsDLVUpdate
}

// ErrNotSupported is returned for capability-gated ops (e.g. SuspendInferior on Delve).
var ErrNotSupported = errNotSupported{}

type errNotSupported struct{}

func (errNotSupported) Error() string { return "backend: not supported" }

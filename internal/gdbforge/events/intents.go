package events

import "github.com/yairgd/gdbforge/internal/gdbforge/models"

// UI intent messages: widgets publish; cmd/gdbforge controllers subscribe.

type CodeBreakToggleMsg struct {
	Path string
	Line int
}

func (CodeBreakToggleMsg) Type() string { return "CodeBreakToggleMsg" }

type CodeBreakEnableToggleMsg struct {
	Path string
	Line int
}

func (CodeBreakEnableToggleMsg) Type() string { return "CodeBreakEnableToggleMsg" }

type AsmBreakToggleMsg struct {
	Addr string
}

func (AsmBreakToggleMsg) Type() string { return "AsmBreakToggleMsg" }

type AsmBreakEnableToggleMsg struct{}

func (AsmBreakEnableToggleMsg) Type() string { return "AsmBreakEnableToggleMsg" }

type AsmBrowseMsg struct {
	Addr string
	Rows int
}

func (AsmBrowseMsg) Type() string { return "AsmBrowseMsg" }

type BreakpointToggleMsg struct {
	Index int
}

func (BreakpointToggleMsg) Type() string { return "BreakpointToggleMsg" }

type BreakpointDeleteMsg struct {
	Index int
}

func (BreakpointDeleteMsg) Type() string { return "BreakpointDeleteMsg" }

type BreakpointActivateMsg struct {
	BP        models.BreakInfo
	FocusCode bool
}

func (BreakpointActivateMsg) Type() string { return "BreakpointActivateMsg" }

type ThreadActivateMsg struct {
	Thread models.ThreadInfo
}

func (ThreadActivateMsg) Type() string { return "ThreadActivateMsg" }

type CallStackActivateMsg struct {
	Frame     models.StackFrame
	FocusCode bool
}

func (CallStackActivateMsg) Type() string { return "CallStackActivateMsg" }

type OpenSourceMsg struct {
	Path string
}

func (OpenSourceMsg) Type() string { return "OpenSourceMsg" }

type FocusCodeMsg struct{}

func (FocusCodeMsg) Type() string { return "FocusCodeMsg" }

type ExecClosedMsg struct{}

func (ExecClosedMsg) Type() string { return "ExecClosedMsg" }

type ExecDismissedMsg struct{}

func (ExecDismissedMsg) Type() string { return "ExecDismissedMsg" }

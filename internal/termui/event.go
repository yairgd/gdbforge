package termui

type CommandID int

const CmdUnknown CommandID = 0
const CmdExitMode CommandID = 1

type SubmitMsg struct {
	Text  string
	CmdID CommandID
	Args  string
}

func (m SubmitMsg) Type() string { return "SubmitMsg" }

// CompletionMsg is delivered on the UI thread (PostInterrupt → EventBus) when
// Tab requests completions. DebuggerApp applies it to CompletionMenu and syncs
// the CompletionView.
type CompletionMsg struct {
	Input string
	Token string
	Names []string
}

func (m CompletionMsg) Type() string { return "CompletionMsg" }

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

// SearchTextChangedMsg is delivered on the UI thread when '/' search cmdline text
// edits (live preview). searchCtl subscribes and updates the SearchHost pattern.
type SearchTextChangedMsg struct {
	Text string
}

func (m SearchTextChangedMsg) Type() string { return "SearchTextChangedMsg" }

// SearchSubmittedMsg is delivered on the UI thread when Enter commits '/' search.
type SearchSubmittedMsg struct {
	Pattern string
}

func (m SearchSubmittedMsg) Type() string { return "SearchSubmittedMsg" }

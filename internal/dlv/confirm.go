package dlv

import "strings"

// ConfirmKind is the kind of interactive answer Delve waits for on the CLI PTY.
type ConfirmKind int

const (
	ConfirmYesNo ConfirmKind = iota
	ConfirmPauseQuit
)

// ConfirmGate tracks Delve CLI interactive prompts (e.g. [Y/n]? after exit,
// [p/q]? after SIGINT in multiclient mode). Peer of gdb.QuitGate.
type ConfirmGate struct {
	confirming bool
	host       string
	kind       ConfirmKind
}

// Confirming is true while waiting for an interactive answer on the Delve PTY.
func (g *ConfirmGate) Confirming() bool {
	return g != nil && g.confirming
}

// Kind is the active confirm prompt kind (ConfirmYesNo when unset).
func (g *ConfirmGate) Kind() ConfirmKind {
	if g == nil || !g.confirming {
		return ConfirmYesNo
	}
	return g.kind
}

// Host is the live input line text for the confirm question (may be empty).
func (g *ConfirmGate) Host() string {
	if g == nil {
		return ""
	}
	return g.host
}

// Begin marks confirming with the given live host string.
func (g *ConfirmGate) Begin(host string) {
	if g == nil {
		return
	}
	g.confirming = true
	g.host = host
}

// Clear ends confirming.
func (g *ConfirmGate) Clear() {
	if g == nil {
		return
	}
	g.confirming = false
	g.host = ""
	g.kind = ConfirmYesNo
}

// Observe updates confirm state from a parsed Delve update.
func (g *ConfirmGate) Observe(u Update) {
	if g == nil {
		return
	}
	if u.ConfirmReady {
		g.confirming = true
		if u.ConfirmHost != "" {
			g.host = u.ConfirmHost
		}
		g.kind = u.ConfirmKind
	}
	if u.PromptReady {
		g.confirming = false
		g.host = ""
		g.kind = ConfirmYesNo
	}
}

// LooksLikeYesNoPrompt reports Delve interactive [Y/n]? / [y/n]? questions.
func LooksLikeYesNoPrompt(s string) bool {
	kind, ok := LooksLikeConfirmPrompt(s)
	return ok && kind == ConfirmYesNo
}

// LooksLikeConfirmPrompt reports Delve [Y/n]? or multiclient SIGINT [p/q]? prompts.
func LooksLikeConfirmPrompt(s string) (ConfirmKind, bool) {
	plain := strings.TrimSpace(s)
	if plain == "" {
		return 0, false
	}
	lower := strings.ToLower(plain)
	if strings.Contains(lower, "[p/q]?") {
		return ConfirmPauseQuit, true
	}
	if strings.Contains(lower, "[y/n]?") {
		return ConfirmYesNo, true
	}
	return 0, false
}

// ConfirmLiveHost returns a walking-prompt host string with one trailing space.
func ConfirmLiveHost(question string) string {
	q := strings.TrimRight(strings.TrimSpace(question), " ")
	if q == "" {
		return ""
	}
	return q + " "
}

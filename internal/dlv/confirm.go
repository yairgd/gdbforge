package dlv

import "strings"

// ConfirmGate tracks Delve CLI yes/no prompts (e.g. suspended breakpoint after exit).
// Peer of gdb.QuitGate for interactive confirmations that are not a bare (dlv) prompt.
type ConfirmGate struct {
	confirming bool
	host       string
}

// Confirming is true while waiting for a y/n answer on the Delve PTY.
func (g *ConfirmGate) Confirming() bool {
	return g != nil && g.confirming
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
	}
	if u.PromptReady {
		g.confirming = false
		g.host = ""
	}
}

// LooksLikeYesNoPrompt reports Delve interactive [Y/n]? / [y/n]? questions.
func LooksLikeYesNoPrompt(s string) bool {
	plain := strings.TrimSpace(s)
	if plain == "" {
		return false
	}
	lower := strings.ToLower(plain)
	return strings.Contains(lower, "[y/n]?")
}

// ConfirmLiveHost returns a walking-prompt host string with one trailing space.
func ConfirmLiveHost(question string) string {
	q := strings.TrimRight(strings.TrimSpace(question), " ")
	if q == "" {
		return ""
	}
	return q + " "
}

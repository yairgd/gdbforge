package dlv

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdb"
	"github.com/yairgd/gdbforge/internal/platform"
)

const (
	completeIdle   = 80 * time.Millisecond
	completeMax    = 1500 * time.Millisecond
	completeDrain  = 20 * time.Millisecond
	maxFuncMatches = 200
	// Avoid dumping the whole binary on a bare "break " Tab.
	minLocspecToken = 1
)

// Commands is a static list of Delve CLI command names for Tab completion.
var Commands = []string{
	"args",
	"b",
	"break",
	"breakpoints",
	"call",
	"check",
	"clear",
	"clearall",
	"condition",
	"config",
	"continue",
	"deferred",
	"disassemble",
	"down",
	"edit",
	"exit",
	"frame",
	"funcs",
	"goroutine",
	"goroutines",
	"help",
	"libraries",
	"list",
	"locals",
	"next",
	"on",
	"print",
	"regs",
	"restart",
	"set",
	"source",
	"sources",
	"stack",
	"step",
	"stepout",
	"thread",
	"threads",
	"trace",
	"types",
	"up",
	"vars",
	"whatis",
}

// Complete runs Delve console Tab completion: command names, or function
// locspecs for break/b/trace/… via `funcs <regex>`.
func Complete(sess core.Session, state *platform.AppState, prefix string) gdb.CompleteResult {
	base := gdb.CompletionBase(prefix)
	if base == "" {
		return CompleteCommands(prefix)
	}
	cmd := firstWord(base)
	if !isLocspecCmd(cmd) {
		return gdb.CompleteResult{}
	}
	token := prefix[len(base):]
	if looksLikeFileLine(token) {
		return gdb.CompleteResult{}
	}
	if len(token) < minLocspecToken {
		return gdb.CompleteResult{}
	}
	if sess == nil {
		return gdb.CompleteResult{}
	}
	raw, err := querySilent(sess, state, "funcs "+funcsRegex(token))
	if err != nil || raw == "" {
		return gdb.CompleteResult{}
	}
	names := ParseFuncs(raw)
	names = filterPrefix(names, token)
	if len(names) > maxFuncMatches {
		names = names[:maxFuncMatches]
	}
	if len(names) == 0 {
		return gdb.CompleteResult{}
	}
	full := make([]string, len(names))
	for i, n := range names {
		full[i] = base + n
	}
	lcp := longestCommonPrefix(names)
	return gdb.CompleteResult{
		Completion: base + lcp,
		Matches:    full,
	}
}

// CompleteCommands completes the first word of prefix against Commands.
func CompleteCommands(prefix string) gdb.CompleteResult {
	base := gdb.CompletionBase(prefix)
	token := prefix[len(base):]
	if base != "" {
		return gdb.CompleteResult{}
	}
	token = strings.TrimLeft(token, " \t")
	var matches []string
	for _, cmd := range Commands {
		if strings.HasPrefix(cmd, token) {
			matches = append(matches, cmd)
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return gdb.CompleteResult{}
	}
	lcp := longestCommonPrefix(matches)
	completion := base + lcp
	full := make([]string, len(matches))
	for i, m := range matches {
		full[i] = base + m
	}
	return gdb.CompleteResult{Completion: completion, Matches: full}
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func isLocspecCmd(cmd string) bool {
	switch strings.ToLower(cmd) {
	case "b", "break", "trace", "clear", "clearall", "list", "edit", "on", "condition":
		return true
	default:
		return false
	}
}

// looksLikeFileLine is true for filename:line locspecs (not package.func).
func looksLikeFileLine(token string) bool {
	i := strings.LastIndexByte(token, ':')
	if i < 0 {
		return false
	}
	// Go package paths use '.' not typically "file.go:N" — if after ':' is empty
	// or all digits, treat as file:line.
	after := token[i+1:]
	if after == "" {
		return true
	}
	for _, r := range after {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func funcsRegex(token string) string {
	// Prefix match: QuoteMeta so "main." → ^main\.
	return "^" + regexp.QuoteMeta(token)
}

// ParseFuncs extracts function names from `funcs` CLI output.
func ParseFuncs(raw string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(raw, "\n") {
		plain := strings.TrimSpace(platform.StripANSI(line))
		plain = strings.TrimRight(plain, "\r")
		if plain == "" || plain == PromptToken {
			continue
		}
		if strings.HasPrefix(plain, "Command failed") {
			continue
		}
		// Skip pager / help chrome.
		if strings.Contains(plain, "functions matching") {
			continue
		}
		// Function names are identifiers / paths with dots; reject prose.
		if strings.ContainsAny(plain, " \t") {
			continue
		}
		if _, ok := seen[plain]; ok {
			continue
		}
		seen[plain] = struct{}{}
		out = append(out, plain)
	}
	sort.Strings(out)
	return out
}

func filterPrefix(names []string, token string) []string {
	if token == "" {
		return names
	}
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, token) {
			out = append(out, n)
		}
	}
	return out
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		for len(p) > 0 && !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
		}
		if p == "" {
			return ""
		}
	}
	return p
}

func querySilent(sess core.Session, state *platform.AppState, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), completeMax)
	defer cancel()

	ch, unsub := sess.Subscribe()
	defer unsub()
	drainComplete(ch, completeDrain)

	var out strings.Builder
	run := func() error {
		return sess.WithWrite(ctx, func(w core.PTYWriter) error {
			if err := w.Send(command); err != nil {
				return err
			}
			captureComplete(ctx, ch, &out, completeIdle, completeMax)
			return nil
		})
	}
	var err error
	if state != nil {
		state.WithPTYOwner(platform.PTYOwnerApp, func() {
			err = run()
		})
	} else {
		err = run()
	}
	return out.String(), err
}

func drainComplete(ch <-chan core.PtyOutputMsg, wait time.Duration) {
	core.Drain(ch, wait)
}

func captureComplete(ctx context.Context, ch <-chan core.PtyOutputMsg, out *strings.Builder, idle, max time.Duration) {
	deadline := time.NewTimer(max)
	defer deadline.Stop()
	idleT := time.NewTimer(idle)
	defer idleT.Stop()

	resetIdle := func() {
		if !idleT.Stop() {
			select {
			case <-idleT.C:
			default:
			}
		}
		idleT.Reset(idle)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-idleT.C:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Err != nil {
				return
			}
			if msg.Data != "" {
				out.WriteString(msg.Data)
				if strings.Contains(msg.Data, PromptToken) {
					return
				}
				resetIdle()
			}
		}
	}
}

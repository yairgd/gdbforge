package gdb

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/platform"
)

const (
	completeIdle = 80 * time.Millisecond
	completeMax  = 1500 * time.Millisecond
	completeDrain = 20 * time.Millisecond
)

// CompleteResult is the parsed ^done payload from MI -complete.
type CompleteResult struct {
	Completion string   // longest common prefix expansion (may equal the input)
	Matches    []string // full-line candidates
}

// CompletionBase returns the already-typed completed words of prefix, including
// the trailing space (e.g. "delete " / "info "). Empty when completing the
// first word.
func CompletionBase(prefix string) string {
	if i := strings.LastIndexAny(prefix, " \t"); i >= 0 {
		return prefix[:i+1]
	}
	return ""
}

// MenuNames strips already-typed words from full-line MI matches so the
// wildmenu shows only the token being completed
// (e.g. "delete bookmark" → "bookmark" when prefix is "delete ").
func MenuNames(prefix string, matches []string) []string {
	if len(matches) == 0 {
		return nil
	}
	base := CompletionBase(prefix)
	if base == "" {
		out := make([]string, len(matches))
		copy(out, matches)
		return out
	}
	out := make([]string, len(matches))
	for i, m := range matches {
		if strings.HasPrefix(m, base) {
			out[i] = m[len(base):]
		} else {
			out[i] = m
		}
	}
	return out
}

// ApplyMenuChoice rebuilds the full input line from the current prefix and a
// wildmenu token (inverse of MenuNames).
func ApplyMenuChoice(prefix, choice string) string {
	if choice == "" {
		return prefix
	}
	return CompletionBase(prefix) + choice
}

// ParseCompleteResult extracts completion= and matches=[...] from raw PTY text.
func ParseCompleteResult(raw string) CompleteResult {
	var out CompleteResult
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Numeric MI tokens: 42^done,...
		for len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			line = line[1:]
		}
		if !strings.HasPrefix(line, "^done") {
			continue
		}
		if c := ExtractMIField(line, "completion"); c != "" {
			out.Completion = c
		}
		out.Matches = ExtractMIListField(line, "matches")
		return out
	}
	return out
}

// ExtractMIListField parses key=["a","b",...] from an MI result record.
func ExtractMIListField(line, key string) []string {
	prefix := key + "=["
	start := strings.Index(line, prefix)
	if start < 0 {
		return nil
	}
	i := start + len(prefix)
	var out []string
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == ',') {
			i++
		}
		if i >= len(line) || line[i] == ']' {
			break
		}
		if line[i] != '"' {
			break
		}
		i++
		var raw strings.Builder
		escaped := false
		for i < len(line) {
			c := line[i]
			i++
			if escaped {
				raw.WriteByte('\\')
				raw.WriteByte(c)
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				break
			}
			raw.WriteByte(c)
		}
		out = append(out, DecodeMIString(raw.String()))
	}
	return out
}

// QuoteCompleteArg quotes text for -complete when it contains whitespace.
func QuoteCompleteArg(text string) string {
	if text == "" {
		return `""`
	}
	needs := false
	for _, r := range text {
		if unicode.IsSpace(r) || r == '"' || r == '\\' {
			needs = true
			break
		}
	}
	if !needs {
		return text
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// CompleteNames runs MI -complete for prefix and returns match names.
// Same shape as commands.Completer / CommandNode.CompleteArgs.
// Uses PTYOwnerApp so the console paint policy matches other silent queries.
func CompleteNames(sess core.Session, state *platform.AppState, prefix string) []string {
	res := Complete(sess, state, prefix)
	if len(res.Matches) > 0 {
		return res.Matches
	}
	if res.Completion != "" && res.Completion != prefix {
		return []string{res.Completion}
	}
	return nil
}

// Complete runs -complete and returns the full parsed result.
func Complete(sess core.Session, state *platform.AppState, prefix string) CompleteResult {
	if sess == nil {
		return CompleteResult{}
	}
	cmd := "-complete " + QuoteCompleteArg(prefix)
	raw, err := querySilent(sess, state, cmd)
	if err != nil || raw == "" {
		return CompleteResult{}
	}
	return ParseCompleteResult(raw)
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
	deadline := time.After(wait)
	for {
		select {
		case <-ch:
		case <-deadline:
			return
		}
	}
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
				if strings.Contains(msg.Data, MIPromptToken) {
					return
				}
				resetIdle()
			}
		}
	}
}

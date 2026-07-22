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

// CompletingLinespec reports whether prefix is completing after a file: linespec
// (e.g. "break hello.c:").
func CompletingLinespec(prefix string) bool {
	return linespecFileColon(prefix) != ""
}

// linespecFileColon returns the prefix through the linespec file: colon
// (e.g. "break hello.c:" / "display banner.c:"), or "" when not completing
// after a file: location. Skips C++ "::" so "NS::foo" is not treated as file.
func linespecFileColon(prefix string) string {
	base := CompletionBase(prefix)
	token := prefix[len(base):]
	filePart, _, ok := splitLinespecFile(token)
	if !ok || filePart == "" {
		return ""
	}
	return base + filePart
}

// splitLinespecFile splits "file.c:func(sig)" at the linespec colon (not "::").
// filePart includes the trailing ':'.
func splitLinespecFile(token string) (filePart, after string, ok bool) {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] != ':' {
			continue
		}
		// Skip C++ scope "::".
		if i > 0 && token[i-1] == ':' {
			i--
			continue
		}
		if i+1 < len(token) && token[i+1] == ':' {
			continue
		}
		return token[:i+1], token[i+1:], true
	}
	return "", "", false
}

// MenuNames strips already-typed words from full-line MI matches so the
// wildmenu shows only the token being completed
// (e.g. "delete bookmark" → "bookmark" when prefix is "delete ").
// After a linespec file: (e.g. "break banner.c:"), shows only the function
// name and signature ("func(int)"), not "banner.c:func(int)".
func MenuNames(prefix string, matches []string) []string {
	if len(matches) == 0 {
		return nil
	}
	base := CompletionBase(prefix)
	fileColon := ""
	if loc := linespecFileColon(prefix); loc != "" {
		fileColon = loc[len(base):] // "banner.c:"
	}
	out := make([]string, len(matches))
	for i, m := range matches {
		rest := m
		if base != "" && strings.HasPrefix(m, base) {
			rest = m[len(base):]
		}
		if fileColon != "" {
			if strings.HasPrefix(rest, fileColon) {
				rest = rest[len(fileColon):]
			} else if _, after, ok := splitLinespecFile(rest); ok {
				// Match used a different spelling of the file; still show func(+sig).
				rest = after
			}
		}
		out[i] = rest
	}
	return out
}

// ApplyMenuChoice rebuilds the full input line from the current prefix and a
// wildmenu token (inverse of MenuNames).
func ApplyMenuChoice(prefix, choice string) string {
	if choice == "" {
		return prefix
	}
	if loc := linespecFileColon(prefix); loc != "" {
		// Choice may be "foo(int, char *)" from signature enrichment.
		return loc + LinespecFuncName(choice)
	}
	return CompletionBase(prefix) + choice
}

// WithCompletionSpace appends a trailing space after a unique/final completion
// (ju→"jump ") so the next token can be typed. Skips when already spaced or the
// result looks like a directory (ends with /).
func WithCompletionSpace(s string) string {
	if s == "" || strings.HasSuffix(s, " ") || strings.HasSuffix(s, "/") {
		return s
	}
	return s + " "
}

// LinespecFuncName strips a display signature down to the bare function name
// for inserting into a file: linespec ("foo(int)" / "int foo(int)" → "foo").
func LinespecFuncName(choice string) string {
	s := strings.TrimSpace(strings.TrimSuffix(choice, ";"))
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if i := strings.LastIndexAny(s, " \t"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// EnrichLinespecMenuNames replaces bare function names with "name(args)" when
// completing after file: and signatures are available from -symbol-info-functions.
func EnrichLinespecMenuNames(prefix string, menu []string, sigs map[string]string) []string {
	if len(menu) == 0 || len(sigs) == 0 || linespecFileColon(prefix) == "" {
		return menu
	}
	out := make([]string, len(menu))
	for i, name := range menu {
		if sig, ok := sigs[name]; ok && sig != "" {
			out[i] = sig
		} else {
			out[i] = name
		}
	}
	return out
}

// FunctionSignatures runs -symbol-info-functions and returns name → "name(args)".
func FunctionSignatures(sess core.Session, state *platform.AppState) map[string]string {
	if sess == nil {
		return nil
	}
	raw, err := querySilent(sess, state, "-symbol-info-functions")
	if err != nil || raw == "" {
		return nil
	}
	return ParseSymbolInfoFunctions(raw)
}

// ParseSymbolInfoFunctions extracts name → "name(args)" from -symbol-info-functions.
func ParseSymbolInfoFunctions(raw string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "name=\"") {
			continue
		}
		for _, block := range symbolInfoBlocks(line) {
			name := ExtractMIField(block, "name")
			typ := ExtractMIField(block, "type")
			if name != "" {
				out[name] = formatFuncSignature(name, typ)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// symbolInfoBlocks returns each leaf {…} with name="…" (per-symbol objects).
// Advances one rune at a time so nested braces are visited; outer containers
// that hold symbols=[…] are skipped.
func symbolInfoBlocks(line string) []string {
	var blocks []string
	for i := 0; i < len(line); i++ {
		if line[i] != '{' {
			continue
		}
		depth := 0
		end := -1
		for j := i; j < len(line); j++ {
			switch line[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			break
		}
		block := line[i : end+1]
		if strings.Contains(block, `name="`) && !strings.Contains(block, "symbols=[") {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// formatFuncSignature builds a wildmenu label "foo(int, char *)" from MI type
// "int (int, char *)".
func formatFuncSignature(name, typ string) string {
	if typ == "" {
		return name
	}
	if i := strings.IndexByte(typ, '('); i >= 0 {
		return name + typ[i:]
	}
	return name
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

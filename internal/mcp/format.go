package mcp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
)

// gdb_command reply formats.
const (
	FormatConsole = "console"
	FormatMI      = "mi"
	FormatBoth    = "both"
)

// streamRe matches console (~) and target (@) stream records; some probe servers send
// `monitor` output on the target stream.
// resultDataRe is the payload of a ^done record, such as value="5".
var resultDataRe = regexp.MustCompile(`(?m)^\d*\^(?:done|running|connected),(.+)$`)

var streamRe = regexp.MustCompile(`(?m)^[~@]"((?:[^"\\]|\\.)*)"\s*$`)

// ConsoleText joins the console and target streams of an MI reply into the text GDB would
// print at a terminal: no echo, no log stream, no result record, no prompt.
func ConsoleText(raw string) string {
	var b strings.Builder
	for _, m := range streamRe.FindAllStringSubmatch(strings.ReplaceAll(raw, "\r", ""), -1) {
		b.WriteString(parse.MIUnquote(m[1]))
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatReply shapes the raw MI reply of a gdb_command. console (default) is the text only,
// and a command that failed is an error; mi is the raw reply; both returns both.
func FormatReply(raw, format string) (any, error) {
	switch format {
	case "", FormatConsole, FormatBoth, FormatMI:
	default:
		return nil, fmt.Errorf("format must be console, mi or both, not %q", format)
	}
	if format == FormatMI {
		return raw, nil
	}
	if msg, ok := parse.MIError(raw); ok {
		return nil, MIFailure(msg)
	}
	text := ConsoleText(raw)
	if text == "" {
		// MI commands (-data-evaluate-expression, -break-insert, …) answer in the result record.
		if m := resultDataRe.FindStringSubmatch(raw); m != nil {
			text = strings.TrimSpace(m[1])
		} else {
			text = "(no output)"
		}
	}
	if format == FormatBoth {
		return map[string]string{"console": text, "mi": raw}, nil
	}
	return text, nil
}

package luahost

import (
	_ "embed"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

//go:embed api_help.txt
var apiHelpRaw string

// APIHelp returns gdbforge API help lines. Optional topic filters entries (case-insensitive substring match on the API name).
func APIHelp(topic string) []string {
	topic = strings.TrimSpace(strings.ToLower(topic))
	blocks := strings.Split(strings.TrimSpace(apiHelpRaw), "\n\n")
	if topic == "" {
		return flattenHelpBlocks(blocks)
	}
	var out []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		title := block
		if i := strings.IndexByte(block, '\n'); i >= 0 {
			title = block[:i]
		}
		if helpEntryMatchesTopic(title, topic) {
			out = append(out, strings.Split(block, "\n")...)
			out = append(out, "")
		}
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return []string{"no help entries match " + topic}
	}
	return out
}

func helpEntryName(title string) string {
	title = strings.TrimSpace(title)
	switch {
	case strings.HasPrefix(title, "gdbforge."):
		rest := strings.TrimPrefix(title, "gdbforge.")
		if i := strings.IndexAny(rest, "( "); i >= 0 {
			rest = rest[:i]
		}
		return strings.ToLower(rest)
	case strings.HasPrefix(title, "pane."):
		rest := strings.TrimPrefix(title, "pane.")
		if i := strings.IndexAny(rest, "( "); i >= 0 {
			rest = rest[:i]
		}
		return strings.ToLower("pane." + rest)
	default:
		return strings.ToLower(title)
	}
}

func helpEntryMatchesTopic(title, topic string) bool {
	return strings.Contains(helpEntryName(title), topic)
}

func flattenHelpBlocks(blocks []string) []string {
	var out []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		out = append(out, strings.Split(block, "\n")...)
		out = append(out, "")
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func (rt *Runtime) luaHelp(L *lua.LState) int {
	topic := ""
	if L.GetTop() >= 1 {
		topic = strings.TrimSpace(L.ToString(1))
	}
	rt.emitAPIHelp(topic)
	return 0
}

func (rt *Runtime) emitAPIHelp(topic string) {
	for _, line := range APIHelp(topic) {
		rt.emitPrint(line)
	}
}

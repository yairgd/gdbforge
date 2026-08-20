package luahost

import (
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const gdbforgeDot = "gdbforge."

// GdbforgeMembers returns sorted gdbforge.* API names from the live Lua table.
func (rt *Runtime) GdbforgeMembers() []string {
	if rt == nil {
		return nil
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.L == nil {
		return nil
	}
	gf := rt.L.GetGlobal("gdbforge")
	tbl, ok := gf.(*lua.LTable)
	if !ok {
		return nil
	}
	var names []string
	tbl.ForEach(func(k, _ lua.LValue) {
		if s, ok := k.(lua.LString); ok {
			names = append(names, string(s))
		}
	})
	sort.Strings(names)
	return names
}

// CompleteGdbforge completes gdbforge.<member> in a REPL input line.
// Returns the possibly extended line and member-name suffix matches.
func CompleteGdbforge(line string, members []string) (string, []string) {
	if len(members) == 0 {
		return line, nil
	}
	trimEnd := strings.TrimRight(line, " \t")
	if trimEnd == "gdbforge" || strings.HasSuffix(trimEnd, " gdbforge") {
		base := strings.TrimRight(line, " \t")
		return base + ".", []string{"."}
	}
	idx := strings.LastIndex(line, gdbforgeDot)
	if idx < 0 {
		return line, nil
	}
	memberPrefix := line[idx+len(gdbforgeDot):]
	var matches []string
	for _, m := range members {
		if strings.HasPrefix(m, memberPrefix) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return line, nil
	}
	lcp := longestCommonPrefix(matches, len(memberPrefix))
	if len(lcp) > len(memberPrefix) {
		line = line[:idx+len(gdbforgeDot)] + lcp
	}
	return line, matches
}

// ApplyGdbforgeChoice rebuilds the input line after a wildmenu member pick.
func ApplyGdbforgeChoice(line, member string) string {
	if member == "." {
		trim := strings.TrimRight(line, " \t")
		if trim == "gdbforge" || strings.HasSuffix(trim, " gdbforge") {
			return trim + "."
		}
		return line
	}
	idx := strings.LastIndex(line, gdbforgeDot)
	if idx < 0 {
		return line
	}
	return line[:idx+len(gdbforgeDot)] + member
}

func longestCommonPrefix(strs []string, minLen int) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for len(prefix) > 0 && (len(s) < len(prefix) || s[:len(prefix)] != prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	if len(prefix) < minLen {
		return ""
	}
	return prefix
}

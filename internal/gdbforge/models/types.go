package models

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// BreakInfo is one breakpoint row (GDB -break-list or Delve breakpoints).
type BreakInfo struct {
	Number    int
	Enabled   bool
	Condition string // GDB cond=…; empty = unconditional
	File      string // fullname preferred, else file / pending path
	Line      int    // 1-based; 0 for address-only breakpoints
	Addr      string // normalized hex address when known (e.g. "0x401126")
}

// Conditional reports whether this breakpoint has a non-empty condition.
func (it BreakInfo) Conditional() bool {
	return it.Condition != ""
}

// BreakGutter is the per-location view of BreakInfo for Code/Asm gutters.
// Built once from []BreakInfo so widgets keep a single map, not parallel facet maps.
type BreakGutter struct {
	Numbers   []int
	Enabled   bool
	Condition string
}

// Conditional reports whether any enabled BP at this location has a condition.
func (g BreakGutter) Conditional() bool {
	return g.Condition != ""
}

// GuttersByLine indexes breakpoints by 1-based source line.
// Merge: any enabled ⇒ Enabled; Condition from first enabled conditional; collect Numbers.
func GuttersByLine(items []BreakInfo) map[int]BreakGutter {
	out := make(map[int]BreakGutter)
	for _, it := range items {
		if it.Line < 1 {
			continue
		}
		g := out[it.Line]
		if it.Number > 0 {
			g.Numbers = append(g.Numbers, it.Number)
		}
		if it.Enabled {
			g.Enabled = true
			if it.Conditional() && g.Condition == "" {
				g.Condition = it.Condition
			}
		}
		out[it.Line] = g
	}
	return out
}

// GuttersByAddr indexes breakpoints by normalized address.
func GuttersByAddr(items []BreakInfo) map[string]BreakGutter {
	out := make(map[string]BreakGutter)
	for _, it := range items {
		addr := NormalizeAddr(it.Addr)
		if addr == "" {
			continue
		}
		g := out[addr]
		if it.Number > 0 {
			g.Numbers = append(g.Numbers, it.Number)
		}
		if it.Enabled {
			g.Enabled = true
			if it.Conditional() && g.Condition == "" {
				g.Condition = it.Condition
			}
		}
		out[addr] = g
	}
	return out
}

// SameSourcePath reports whether two paths refer to the same source file
// (exact match or basename match).
func SameSourcePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return filepath.Base(a) == filepath.Base(b)
}

// SameSourceLoc reports whether two file:line pairs refer to the same location.
func SameSourceLoc(aFile string, aLine int, bFile string, bLine int) bool {
	if aLine < 1 || bLine < 1 || aLine != bLine {
		return false
	}
	return SameSourcePath(aFile, bFile)
}

// NormalizeAddr parses a hex address and returns a stable "0x%x" form.
func NormalizeAddr(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 64)
	if err != nil {
		return s
	}
	return fmt.Sprintf("0x%x", n)
}

// ThreadInfo is one row from -thread-info / Delve threads.
type ThreadInfo struct {
	ID      string
	State   string
	Name    string
	File    string
	Line    int
	Func    string
	Current bool
}

// StackFrame is one row from -stack-list-frames / Delve stack.
type StackFrame struct {
	Level int
	Func  string
	File  string
	Line  int
	Addr  string
	From  string // shared library path when File is empty (MI "from=")
}

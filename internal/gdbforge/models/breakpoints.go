package models

import (
	"fmt"
	"path/filepath"

	"github.com/yairgd/gdbforge/internal/mcp"
)

// BreakpointList is the shared breakpoint model for GUI and MCP/AI.
// It keeps disabled rows that are absent from GDB. Controllers send the
// returned MI/CLI commands; views only display Items().
type BreakpointList struct {
	items []mcp.BreakInfo
}

// Items returns a copy of all rows (enabled and disabled).
func (b *BreakpointList) Items() []mcp.BreakInfo {
	if b == nil || len(b.items) == 0 {
		return nil
	}
	return append([]mcp.BreakInfo(nil), b.items...)
}

// Enabled returns breakpoints currently active in GDB.
func (b *BreakpointList) Enabled() []mcp.BreakInfo {
	if b == nil {
		return nil
	}
	var out []mcp.BreakInfo
	for _, it := range b.items {
		if it.Enabled {
			out = append(out, it)
		}
	}
	return out
}

// Len returns the number of rows.
func (b *BreakpointList) Len() int {
	if b == nil {
		return 0
	}
	return len(b.items)
}

// At returns the row at i, or false.
func (b *BreakpointList) At(i int) (mcp.BreakInfo, bool) {
	if b == nil || i < 0 || i >= len(b.items) {
		return mcp.BreakInfo{}, false
	}
	return b.items[i], true
}

// HasEnabledAt reports an enabled breakpoint at file:line.
func (b *BreakpointList) HasEnabledAt(file string, line int) bool {
	if b == nil || file == "" || line < 1 {
		return false
	}
	base := filepath.Base(file)
	for _, it := range b.items {
		if !it.Enabled || it.Line != line {
			continue
		}
		if it.File == file || filepath.Base(it.File) == base {
			return true
		}
	}
	return false
}

// MergeFromGDB syncs live GDB breakpoints without dropping locally disabled rows.
func (b *BreakpointList) MergeFromGDB(gdbItems []mcp.BreakInfo) {
	if b == nil {
		return
	}
	keyOf := func(it mcp.BreakInfo) string {
		return fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
	}
	gdbByKey := make(map[string]mcp.BreakInfo, len(gdbItems))
	for _, g := range gdbItems {
		g.Enabled = true
		gdbByKey[keyOf(g)] = g
	}

	placed := make(map[string]bool)
	out := make([]mcp.BreakInfo, 0, len(b.items)+len(gdbItems))
	for _, local := range b.items {
		k := keyOf(local)
		if g, ok := gdbByKey[k]; ok {
			out = append(out, g)
			placed[k] = true
			continue
		}
		if !local.Enabled {
			local.Number = 0
			out = append(out, local)
		}
	}
	for _, g := range gdbItems {
		k := keyOf(g)
		if placed[k] {
			continue
		}
		g.Enabled = true
		out = append(out, g)
		placed[k] = true
	}
	b.items = out
}

// BreakLoc formats file:line for GDB break/clear.
func BreakLoc(it mcp.BreakInfo) string {
	file := it.File
	if file == "" {
		file = "?"
	}
	return fmt.Sprintf("%s:%d", file, it.Line)
}

// ToggleEnableAt disables (delete from GDB, keep row) or re-enables (break).
// Returns the command to send, or "" if nothing to do.
func (b *BreakpointList) ToggleEnableAt(index int) (cmd string, ok bool) {
	if b == nil || index < 0 || index >= len(b.items) {
		return "", false
	}
	it := b.items[index]
	loc := BreakLoc(it)
	if it.Enabled {
		if it.Number > 0 {
			cmd = fmt.Sprintf("-break-delete %d", it.Number)
		} else {
			cmd = "clear " + loc
		}
		it.Enabled = false
		it.Number = 0
		b.items[index] = it
		return cmd, true
	}
	it.Enabled = true
	b.items[index] = it
	return "break " + loc, true
}

// DeleteAt removes the row and returns a GDB delete/clear command when enabled.
func (b *BreakpointList) DeleteAt(index int) (cmd string, ok bool) {
	if b == nil || index < 0 || index >= len(b.items) {
		return "", false
	}
	it := b.items[index]
	if it.Enabled {
		if it.Number > 0 {
			cmd = fmt.Sprintf("-break-delete %d", it.Number)
		} else {
			cmd = "clear " + BreakLoc(it)
		}
	}
	b.items = append(b.items[:index], b.items[index+1:]...)
	return cmd, true
}

// ToggleEnableAtFileLine is BreakpointWidget "e" at file:line (for Code e).
// If codeHasEnabled and there is no row yet, inserts an enabled stub then disables.
func (b *BreakpointList) ToggleEnableAtFileLine(file string, line int, codeHasEnabled bool) (cmd string, index int, ok bool) {
	if b == nil || file == "" || line < 1 {
		return "", -1, false
	}
	base := filepath.Base(file)
	idx := -1
	for i, it := range b.items {
		if it.Line != line {
			continue
		}
		if it.File == file || filepath.Base(it.File) == base {
			idx = i
			break
		}
	}
	if idx < 0 {
		if !codeHasEnabled {
			return "", -1, false
		}
		b.items = append(b.items, mcp.BreakInfo{
			File:    file,
			Line:    line,
			Enabled: true,
		})
		idx = len(b.items) - 1
	}
	cmd, ok = b.ToggleEnableAt(idx)
	return cmd, idx, ok
}

// ToggleInsertClear is Code Space: clear if enabled at location, else break.
// Updates the model optimistically; controller should still refresh from GDB.
func (b *BreakpointList) ToggleInsertClear(file string, line int) (cmd string, ok bool) {
	if b == nil || file == "" || line < 1 {
		return "", false
	}
	loc := fmt.Sprintf("%s:%d", filepath.Base(file), line)
	if b.HasEnabledAt(file, line) {
		b.removeAtFileLine(file, line)
		return "clear " + loc, true
	}
	b.items = append(b.items, mcp.BreakInfo{
		File:    file,
		Line:    line,
		Enabled: true,
	})
	return "break " + loc, true
}

func (b *BreakpointList) removeAtFileLine(file string, line int) {
	base := filepath.Base(file)
	out := b.items[:0]
	for _, it := range b.items {
		if it.Line == line && (it.File == file || filepath.Base(it.File) == base) {
			continue
		}
		out = append(out, it)
	}
	b.items = out
}

// IndexOfFileLine returns the first matching row index, or -1.
func (b *BreakpointList) IndexOfFileLine(file string, line int) int {
	if b == nil {
		return -1
	}
	base := filepath.Base(file)
	for i, it := range b.items {
		if it.Line != line {
			continue
		}
		if it.File == file || filepath.Base(it.File) == base {
			return i
		}
	}
	return -1
}

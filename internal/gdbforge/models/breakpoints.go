package models

import (
	"fmt"
	"path/filepath"
)

// BreakpointList is the shared breakpoint model for GUI and MCP/AI.
// It keeps disabled rows that are absent from GDB. Controllers send the
// returned MI/CLI commands; views only display Items().
type BreakpointList struct {
	items []BreakInfo
}

// Items returns a copy of all rows (enabled and disabled).
func (b *BreakpointList) Items() []BreakInfo {
	if b == nil || len(b.items) == 0 {
		return nil
	}
	return append([]BreakInfo(nil), b.items...)
}

// Clear removes all rows (e.g. UI reset after kill / inferior exit).
func (b *BreakpointList) Clear() {
	if b == nil {
		return
	}
	b.items = nil
}

// Enabled returns breakpoints currently active in GDB.
func (b *BreakpointList) Enabled() []BreakInfo {
	if b == nil {
		return nil
	}
	var out []BreakInfo
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
func (b *BreakpointList) At(i int) (BreakInfo, bool) {
	if b == nil || i < 0 || i >= len(b.items) {
		return BreakInfo{}, false
	}
	return b.items[i], true
}

// HasEnabledAt reports an enabled breakpoint at file:line.
func (b *BreakpointList) HasEnabledAt(file string, line int) bool {
	if b == nil || file == "" || line < 1 {
		return false
	}
	for _, it := range b.items {
		if !it.Enabled || it.Line != line {
			continue
		}
		if SameSourcePath(it.File, file) {
			return true
		}
	}
	return false
}

// MergeFromGDB syncs live GDB breakpoints without dropping locally disabled rows.
func (b *BreakpointList) MergeFromGDB(gdbItems []BreakInfo) {
	if b == nil {
		return
	}
	gdbByKey := make(map[string]BreakInfo, len(gdbItems))
	for _, g := range gdbItems {
		g.Enabled = true
		gdbByKey[breakKey(g)] = g
	}

	placed := make(map[string]bool)
	out := make([]BreakInfo, 0, len(b.items)+len(gdbItems))
	for _, local := range b.items {
		k := breakKey(local)
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
		k := breakKey(g)
		if placed[k] {
			continue
		}
		g.Enabled = true
		out = append(out, g)
		placed[k] = true
	}
	b.items = out
}

func breakKey(it BreakInfo) string {
	if it.File != "" && it.Line > 0 {
		return fmt.Sprintf("%s:%d", filepath.Base(it.File), it.Line)
	}
	if it.Addr != "" {
		return "addr:" + it.Addr
	}
	return fmt.Sprintf("num:%d", it.Number)
}

// BreakLoc formats a location for GDB break/clear.
func BreakLoc(it BreakInfo) string {
	if it.File != "" && it.Line > 0 {
		file := it.File
		return fmt.Sprintf("%s:%d", file, it.Line)
	}
	if it.Addr != "" {
		return "*" + it.Addr
	}
	return "?"
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
		b.items = append(b.items, BreakInfo{
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
	b.items = append(b.items, BreakInfo{
		File:    file,
		Line:    line,
		Enabled: true,
	})
	return "break " + loc, true
}

// ToggleInsertClearAddr is Assembly Space: clear if enabled at addr, else break *addr.
func (b *BreakpointList) ToggleInsertClearAddr(addr string) (cmd string, ok bool) {
	if b == nil || addr == "" {
		return "", false
	}
	loc := "*" + addr
	if idx := b.IndexOfAddr(addr); idx >= 0 {
		it := b.items[idx]
		if it.Enabled {
			if it.Number > 0 {
				cmd = fmt.Sprintf("-break-delete %d", it.Number)
			} else {
				cmd = "clear " + loc
			}
			b.items = append(b.items[:idx], b.items[idx+1:]...)
			return cmd, true
		}
		it.Enabled = true
		b.items[idx] = it
		return "break " + loc, true
	}
	b.items = append(b.items, BreakInfo{
		Addr:    addr,
		Enabled: true,
	})
	return "break " + loc, true
}

// HasEnabledAtAddr reports an enabled breakpoint at the given address.
func (b *BreakpointList) HasEnabledAtAddr(addr string) bool {
	if b == nil || addr == "" {
		return false
	}
	for _, it := range b.items {
		if it.Enabled && it.Addr == addr {
			return true
		}
	}
	return false
}

// HasAsmAndCodeAt reports an enabled breakpoint at file:line that also has an
// address (visible in both Code and Assembly at the same $pc).
func (b *BreakpointList) HasAsmAndCodeAt(file string, line int) bool {
	_, ok := b.AsmAndCodeAt(file, line)
	return ok
}

// AsmAndCodeAt returns an enabled breakpoint at file:line with a non-empty Addr.
func (b *BreakpointList) AsmAndCodeAt(file string, line int) (BreakInfo, bool) {
	if b == nil || file == "" || line < 1 {
		return BreakInfo{}, false
	}
	base := filepath.Base(file)
	for _, it := range b.items {
		if !it.Enabled || it.Line != line || it.Addr == "" {
			continue
		}
		if it.File == file || filepath.Base(it.File) == base {
			return it, true
		}
	}
	return BreakInfo{}, false
}

// SourceAtAddr returns an enabled breakpoint at addr that also has file:line
// (same $pc known in Assembly and Code).
func (b *BreakpointList) SourceAtAddr(addr string) (BreakInfo, bool) {
	if b == nil || addr == "" {
		return BreakInfo{}, false
	}
	for _, it := range b.items {
		if !it.Enabled || it.Addr != addr {
			continue
		}
		if it.File != "" && it.Line > 0 {
			return it, true
		}
	}
	return BreakInfo{}, false
}

// IndexOfAddr returns the first row with matching Addr, or -1.
func (b *BreakpointList) IndexOfAddr(addr string) int {
	if b == nil || addr == "" {
		return -1
	}
	for i, it := range b.items {
		if it.Addr == addr {
			return i
		}
	}
	return -1
}

// ToggleEnableAtAddr is Assembly "e" at the browse address.
func (b *BreakpointList) ToggleEnableAtAddr(addr string, hasEnabled bool) (cmd string, index int, ok bool) {
	if b == nil || addr == "" {
		return "", -1, false
	}
	idx := b.IndexOfAddr(addr)
	if idx < 0 {
		if !hasEnabled {
			return "", -1, false
		}
		b.items = append(b.items, BreakInfo{
			Addr:    addr,
			Enabled: true,
		})
		idx = len(b.items) - 1
	}
	cmd, ok = b.ToggleEnableAt(idx)
	return cmd, idx, ok
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

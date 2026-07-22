package models

import "github.com/yairgd/gdbforge/internal/mcp"

// ThreadList is the shared thread snapshot for GUI and MCP/AI.
type ThreadList struct {
	items []mcp.ThreadInfo
}

// Set replaces the list from a GDB -thread-info parse.
func (t *ThreadList) Set(items []mcp.ThreadInfo) {
	if t == nil {
		return
	}
	t.items = append([]mcp.ThreadInfo(nil), items...)
}

// Items returns a copy of the current threads.
func (t *ThreadList) Items() []mcp.ThreadInfo {
	if t == nil || len(t.items) == 0 {
		return nil
	}
	return append([]mcp.ThreadInfo(nil), t.items...)
}

// Len returns the number of threads.
func (t *ThreadList) Len() int {
	if t == nil {
		return 0
	}
	return len(t.items)
}

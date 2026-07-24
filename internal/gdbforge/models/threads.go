package models

// ThreadList is the shared thread snapshot for GUI and MCP/AI.
type ThreadList struct {
	items []ThreadInfo
}

// Set replaces the list from a GDB -thread-info parse.
func (t *ThreadList) Set(items []ThreadInfo) {
	if t == nil {
		return
	}
	t.items = append([]ThreadInfo(nil), items...)
}

// Items returns a copy of the current threads.
func (t *ThreadList) Items() []ThreadInfo {
	if t == nil || len(t.items) == 0 {
		return nil
	}
	return append([]ThreadInfo(nil), t.items...)
}

// Len returns the number of threads.
func (t *ThreadList) Len() int {
	if t == nil {
		return 0
	}
	return len(t.items)
}

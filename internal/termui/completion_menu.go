package termui

// CompletionMenu is the wildmenu model: candidate set and selection.
// Views (bar, future list window) only paint Snapshots.
// Filtering is not done here — the owner re-queries completions whenever
// the source line (GDB console or cmdline) changes.
type CompletionMenu struct {
	names    []string
	selected int
}

// Set replaces candidates. len <= 1 clears the menu (no wildmenu for 0/1 match).
func (m *CompletionMenu) Set(names []string) {
	if len(names) <= 1 {
		m.Clear()
		return
	}
	m.names = append([]string(nil), names...)
	m.selected = 0
}

// Clear drops candidates.
func (m *CompletionMenu) Clear() {
	m.names = nil
	m.selected = 0
}

// Active reports whether there are visible candidates.
func (m *CompletionMenu) Active() bool {
	return m != nil && len(m.names) > 0
}

// Visible returns the current candidate list (copy).
func (m *CompletionMenu) Visible() []string {
	if m == nil || len(m.names) == 0 {
		return nil
	}
	return append([]string(nil), m.names...)
}

// SelectedIndex returns the highlight index, or -1.
func (m *CompletionMenu) SelectedIndex() int {
	if m == nil || !m.Active() {
		return -1
	}
	return m.selected
}

// Selected returns the highlighted completion name, or "".
func (m *CompletionMenu) Selected() string {
	if m == nil || m.selected < 0 || m.selected >= len(m.names) {
		return ""
	}
	return m.names[m.selected]
}

// Snapshot returns visible names and selected index for a CompletionView.
func (m *CompletionMenu) Snapshot() (names []string, selected int) {
	if m == nil || !m.Active() {
		return nil, 0
	}
	return m.Visible(), m.selected
}

// Move steps the highlight by delta (wraps).
func (m *CompletionMenu) Move(delta int) {
	if m == nil {
		return
	}
	n := len(m.names)
	if n == 0 {
		return
	}
	m.selected = (m.selected + delta%n + n) % n
}

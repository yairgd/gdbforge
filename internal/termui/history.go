package termui

type History interface {
	Add(cmd string)
	Prev() string
	Next() string
	ResetNavigation()
	Current() string
	SetBuffer(buf string)
}

type MemoryHistory struct {
	items  []string
	index  int
	buffer string
}

func NewMemoryHistory() *MemoryHistory {
	return &MemoryHistory{}
}

func (h *MemoryHistory) Add(cmd string) {
	if h == nil || cmd == "" {
		return
	}
	// Consecutive dedupe (bash ignoredups): same as last entry → skip.
	if n := len(h.items); n > 0 && h.items[n-1] == cmd {
		h.index = n
		h.buffer = ""
		return
	}
	h.items = append(h.items, cmd)
	h.index = len(h.items)
	h.buffer = ""
}

func (h *MemoryHistory) Prev() string {
	if len(h.items) == 0 {
		return h.buffer
	}
	if h.index > 0 {
		h.index--
		return h.items[h.index]
	}
	return h.items[0]
}

func (h *MemoryHistory) Next() string {
	if len(h.items) == 0 {
		return h.buffer
	}
	if h.index < len(h.items)-1 {
		h.index++
		return h.items[h.index]
	}
	h.index = len(h.items)
	return h.buffer
}

func (h *MemoryHistory) ResetNavigation() {
	h.index = len(h.items)
}

func (h *MemoryHistory) SetBuffer(buf string) {
	h.buffer = buf
}

func (h *MemoryHistory) Current() string {
	if h.index < len(h.items) {
		return h.items[h.index]
	}
	return h.buffer
}

// Items returns a copy of the history entries (oldest first).
func (h *MemoryHistory) Items() []string {
	if h == nil || len(h.items) == 0 {
		return nil
	}
	out := make([]string, len(h.items))
	copy(out, h.items)
	return out
}

// Load replaces history with items and resets navigation to the end.
// Consecutive duplicates are collapsed (same rule as Add).
func (h *MemoryHistory) Load(items []string) {
	if h == nil {
		return
	}
	h.items = h.items[:0]
	h.buffer = ""
	for _, s := range items {
		h.Add(s)
	}
	h.index = len(h.items)
	h.buffer = ""
}

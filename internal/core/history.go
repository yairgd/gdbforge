package core

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
	index  int    // current position in history
	buffer string // current editable buffer (not yet committed)
}

// --- constructor ---

func NewMemoryHistory() *MemoryHistory {
	return &MemoryHistory{
		items:  []string{},
		index:  0,
		buffer: "",
	}
}

// --- add command to history ---

func (h *MemoryHistory) Add(cmd string) {
	if cmd == "" {
		return
	}

	h.items = append(h.items, cmd)
	h.index = len(h.items)
	h.buffer = ""
}

// --- navigate up (older commands) ---

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

// --- navigate down (newer commands) ---

func (h *MemoryHistory) Next() string {
	if len(h.items) == 0 {
		return h.buffer
	}

	if h.index < len(h.items)-1 {
		h.index++
		return h.items[h.index]
	}

	// back to empty input
	h.index = len(h.items)
	return h.buffer
}

// --- reset navigation (after typing new text) ---

func (h *MemoryHistory) ResetNavigation() {
	h.index = len(h.items)
}

// --- store current editing buffer (important!) ---

func (h *MemoryHistory) SetBuffer(buf string) {
	h.buffer = buf
}

// --- get current buffer ---

func (h *MemoryHistory) Current() string {
	if h.index < len(h.items) {
		return h.items[h.index]
	}
	return h.buffer
}

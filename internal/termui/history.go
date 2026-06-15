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
	if cmd == "" {
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

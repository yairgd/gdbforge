package serialmux

import "sync"

var registry struct {
	mu sync.Mutex
	m  map[string]*Mux
}

func register(device string, m *Mux) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.m == nil {
		registry.m = make(map[string]*Mux)
	}
	registry.m[device] = m
}

func release(device string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.m, device)
}

func claim(device string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.m == nil {
		return nil
	}
	if _, ok := registry.m[device]; ok {
		return ErrAlreadyOpen
	}
	return nil
}

// Lookup returns the open mux for device, if any.
func Lookup(device string) *Mux {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.m == nil {
		return nil
	}
	return registry.m[device]
}

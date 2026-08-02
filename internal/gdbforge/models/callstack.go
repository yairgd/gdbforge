package models

// CallStack is the shared stack-frame snapshot for GUI and MCP/AI.
type CallStack struct {
	frames []StackFrame
}

// Set replaces frames from a GDB -stack-list-frames parse.
func (c *CallStack) Set(frames []StackFrame) {
	if c == nil {
		return
	}
	c.frames = append([]StackFrame(nil), frames...)
}

// Items returns a copy of the current frames.
func (c *CallStack) Items() []StackFrame {
	if c == nil || len(c.frames) == 0 {
		return nil
	}
	return append([]StackFrame(nil), c.frames...)
}

// Len returns the number of frames.
func (c *CallStack) Len() int {
	if c == nil {
		return 0
	}
	return len(c.frames)
}

// FirstWithFile returns the topmost frame that has a source file, or false.
func (c *CallStack) FirstWithFile() (StackFrame, bool) {
	if c == nil {
		return StackFrame{}, false
	}
	for _, fr := range c.frames {
		if fr.File != "" {
			return fr, true
		}
	}
	return StackFrame{}, false
}

// At returns the frame at i, or false.
func (c *CallStack) At(i int) (StackFrame, bool) {
	if c == nil || i < 0 || i >= len(c.frames) {
		return StackFrame{}, false
	}
	return c.frames[i], true
}

// ByLevel returns the frame with the given GDB/Delve level, or false.
func (c *CallStack) ByLevel(level int) (StackFrame, bool) {
	if c == nil {
		return StackFrame{}, false
	}
	for _, fr := range c.frames {
		if fr.Level == level {
			return fr, true
		}
	}
	return StackFrame{}, false
}

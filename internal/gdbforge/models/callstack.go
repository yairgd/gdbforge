package models

import "github.com/yairgd/gdbforge/internal/mcp"

// CallStack is the shared stack-frame snapshot for GUI and MCP/AI.
type CallStack struct {
	frames []mcp.StackFrame
}

// Set replaces frames from a GDB -stack-list-frames parse.
func (c *CallStack) Set(frames []mcp.StackFrame) {
	if c == nil {
		return
	}
	c.frames = append([]mcp.StackFrame(nil), frames...)
}

// Items returns a copy of the current frames.
func (c *CallStack) Items() []mcp.StackFrame {
	if c == nil || len(c.frames) == 0 {
		return nil
	}
	return append([]mcp.StackFrame(nil), c.frames...)
}

// Len returns the number of frames.
func (c *CallStack) Len() int {
	if c == nil {
		return 0
	}
	return len(c.frames)
}

// FirstWithFile returns the topmost frame that has a source file, or false.
func (c *CallStack) FirstWithFile() (mcp.StackFrame, bool) {
	if c == nil {
		return mcp.StackFrame{}, false
	}
	for _, fr := range c.frames {
		if fr.File != "" {
			return fr, true
		}
	}
	return mcp.StackFrame{}, false
}

// At returns the frame at i, or false.
func (c *CallStack) At(i int) (mcp.StackFrame, bool) {
	if c == nil || i < 0 || i >= len(c.frames) {
		return mcp.StackFrame{}, false
	}
	return c.frames[i], true
}

// ByLevel returns the frame with the given GDB/Delve level, or false.
func (c *CallStack) ByLevel(level int) (mcp.StackFrame, bool) {
	if c == nil {
		return mcp.StackFrame{}, false
	}
	for _, fr := range c.frames {
		if fr.Level == level {
			return fr, true
		}
	}
	return mcp.StackFrame{}, false
}

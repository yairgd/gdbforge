package widgets

import "github.com/yairgd/gdbforge/internal/gdbforge/models"

// Test stubs implementing widget host interfaces.

type stubBreakpointHost struct {
	activate  func(models.BreakInfo)
	focusCode func()
	toggle    func(int)
	delete    func(int)
}

func (s stubBreakpointHost) ActivateBreakpoint(bp models.BreakInfo) {
	if s.activate != nil {
		s.activate(bp)
	}
}
func (s stubBreakpointHost) FocusCode() {
	if s.focusCode != nil {
		s.focusCode()
	}
}
func (s stubBreakpointHost) ToggleBreakpoint(index int) {
	if s.toggle != nil {
		s.toggle(index)
	}
}
func (s stubBreakpointHost) DeleteBreakpoint(index int) {
	if s.delete != nil {
		s.delete(index)
	}
}

type stubThreadHost struct {
	activate func(models.ThreadInfo)
}

func (s stubThreadHost) ActivateThread(th models.ThreadInfo) {
	if s.activate != nil {
		s.activate(th)
	}
}

type stubCallStackHost struct {
	activate  func(models.StackFrame)
	focusCode func()
}

func (s stubCallStackHost) ActivateCallStack(fr models.StackFrame) {
	if s.activate != nil {
		s.activate(fr)
	}
}
func (s stubCallStackHost) FocusCode() {
	if s.focusCode != nil {
		s.focusCode()
	}
}

type stubFileListHost struct {
	open func(string)
}

func (s stubFileListHost) OpenSourcePath(path string) {
	if s.open != nil {
		s.open(path)
	}
}

type stubAssemblyHost struct {
	browse       func(string, int)
	toggleBreak  func(string)
	toggleEnable func()
}

func (s stubAssemblyHost) BrowseAssembly(addr string, rows int) {
	if s.browse != nil {
		s.browse(addr, rows)
	}
}
func (s stubAssemblyHost) ToggleAsmBreak(addr string) {
	if s.toggleBreak != nil {
		s.toggleBreak(addr)
	}
}
func (s stubAssemblyHost) ToggleAsmBreakEnable() {
	if s.toggleEnable != nil {
		s.toggleEnable()
	}
}

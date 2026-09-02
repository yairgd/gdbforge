package main

import (
	"github.com/yairgd/gdbforge/internal/gdbforge/debugger"
)

func (a *DebuggerApp) InferiorMode() debugger.InferiorMode {
	if a.backend != nil && a.backend.UsesExternalInferiorTTY() {
		return debugger.InferiorExternal
	}
	return debugger.InferiorInternal
}

func (a *DebuggerApp) InferiorTTYPath() string {
	if a.backend == nil {
		return ""
	}
	return a.backend.InferiorTTYPath()
}

func (a *DebuggerApp) SetInferiorInternal() error {
	return a.SetInferiorTTY("internal")
}

func (a *DebuggerApp) SetInferiorExternal() error {
	return a.SetInferiorTTY("external")
}

func (a *DebuggerApp) SetInferiorPath(path string) error {
	return a.SetInferiorTTY(path)
}

func (a *DebuggerApp) syncInferiorIOView() {
	if a.backend == nil {
		return
	}
	if a.backend.UsesExternalInferiorTTY() {
		a.inferiorIO.markExternal(a.backend.InferiorTTYPath())
	} else if inf := a.backend.InferiorTTY(); inf != nil {
		a.inferiorIO.rewireInternal(inf)
	}
}

var _ debugger.InferiorIO = (*DebuggerApp)(nil)

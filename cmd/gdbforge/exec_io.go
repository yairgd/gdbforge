package main

import (
	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/core"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/platform"
)

type execHost interface {
	ExecWidget() *widgets.ExecWidget
}

// execIOCtl routes :! exec PTY chunks to ExecWidget on the UI thread.
type execIOCtl struct {
	host execHost
}

func (c *execIOCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onOutput)
}

func (c *execIOCtl) onOutput(msg core.ExecOutputMsg) {
	h := c.host
	if h == nil {
		return
	}
	w := h.ExecWidget()
	if w == nil {
		return
	}
	w.HandleEvent(tcell.NewEventInterrupt(msg))
}

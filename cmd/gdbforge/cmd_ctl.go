package main

import (
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

type cmdHost interface {
	leaveCommandMode()
	tryGotoLineCmd(text string) bool
}

// cmdCtl handles cmdline SubmitMsg events (Esc / unknown submit / goto-line).
type cmdCtl struct {
	host cmdHost
}

func (c *cmdCtl) Register(bus *platform.EventBus) {
	platform.Subscribe(bus, c.onSubmit)
}

func (c *cmdCtl) onSubmit(msg termui.SubmitMsg) {
	if c.host == nil {
		return
	}
	switch msg.CmdID {
	case termui.CmdExitMode:
		c.host.leaveCommandMode()
	case termui.CmdUnknown:
		c.host.tryGotoLineCmd(msg.Text)
	}
}

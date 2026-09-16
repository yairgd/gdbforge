package main

import (
	"github.com/yairgd/termforge"
	"github.com/yairgd/termforge/platform"
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

func (c *cmdCtl) onSubmit(msg termforge.SubmitMsg) {
	if c.host == nil {
		return
	}
	switch msg.CmdID {
	case termforge.CmdExitMode:
		c.host.leaveCommandMode()
	case termforge.CmdUnknown:
		c.host.tryGotoLineCmd(msg.Text)
	}
}

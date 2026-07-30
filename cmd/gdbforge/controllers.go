package main

// breakCtl owns breakpoint send/sync/toggle intents. Wired as
// bpWidget.OnActivate = a.breaks.Activate (methods live on breakCtl, not DebuggerApp).
type breakCtl struct{ app *DebuggerApp }

// navCtl owns call-stack / thread activate intents.
type navCtl struct{ app *DebuggerApp }

func (a *DebuggerApp) initControllers() {
	a.breaks.app = a
	a.nav.app = a
}


func (c *breakCtl) Toggle(index int) { c.onBreakpointToggle(index) }
func (c *breakCtl) Delete(index int) { c.onBreakpointDelete(index) }
func (c *breakCtl) SyncViews()       { c.syncBreakpointViews() }
func (c *breakCtl) SendCmd(cmd string) { c.sendBreakpointCmd(cmd) }



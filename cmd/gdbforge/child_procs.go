package main

import "os/exec"

// trackStartedCmd registers a freshly started child process with the app's
// termforge.ChildProcCtl so shutdown reaps it. The tracker itself is generic
// and lives in termforge; only this binding to DebuggerApp is app-specific.
func (a *DebuggerApp) trackStartedCmd(cmd *exec.Cmd, killGroup bool) {
	if a == nil || cmd == nil || cmd.Process == nil {
		return
	}
	a.children.Track(cmd.Process.Pid, killGroup)
}

package main

import (
	"github.com/yairgd/cgdb-go/internal/commands"
)

// ExapData builds the command hierarchy on commandReg.Root:
//
//	/ → window → left, right, up, down
//	/ → gdb → break → file, delete
//	/ → gdb → info → registers, threads
//	/ → ! <cmdline>  (Vim-style :!bash / :!ls — ExecClient + ExecWidget)
//	/ → AI <question> / ai <question>  (in-app LLM on live GDB)
//	/ → set → equalalways, noequalalways, clearoutput, noclearoutput,
//	          continueafterclear, nocontinueafterclear, esctocode, noesctocode,
//	          markcolor, breakcolor, breakdisabledcolor
//	/ → layout <name>  (apply named workspace layout)
//	/ → b <name>   (switch buffer: about, logger, gdb, exec, or open file)
//	/ → edit [name]  (project source picker, or open a source file; :e is unique prefix)
//	/ → vs, split, clear, quit
func (a *DebuggerApp) ExapData() {
	a.commandReg.Root.
		Group("window",
			commands.Cmd("left", a.OnFocusLeft),
			commands.Cmd("right", a.OnFocusRight),
			commands.Cmd("up", a.OnFocusUp),
			commands.Cmd("down", a.OnFocusDown),
		).
		Group("gdb",
			commands.Group("break",
				commands.Cmd("file", a.BreakFile),
				commands.Cmd("delete", a.DeleteBreakpoint),
			),
			commands.Group("info",
				commands.Cmd("registers", a.ShowRegisters),
				commands.Cmd("threads", a.ShowThreads),
			),
		).
		LeafRest("!", a.OnRun).
		LeafRest("AI", a.OnAI).
		LeafRest("ai", a.OnAI).
		Group("set",
			commands.Cmd("equalalways", a.SetEqualAlwaysOn),
			commands.Cmd("noequalalways", a.SetEqualAlwaysOff),
			commands.Cmd("clearoutput", a.SetClearOutputOn),
			commands.Cmd("noclearoutput", a.SetClearOutputOff),
			commands.Cmd("continueafterclear", a.SetContinueAfterClearOn),
			commands.Cmd("nocontinueafterclear", a.SetContinueAfterClearOff),
			commands.Cmd("esctocode", a.SetEscToCodeOn),
			commands.Cmd("noesctocode", a.SetEscToCodeOff),
			commands.CmdRest("markcolor", a.SetMarkColor),
			commands.CmdRest("breakcolor", a.SetBreakColor),
			commands.CmdRest("breakdisabledcolor", a.SetBreakDisabledColor),
		).
		LeafRestComplete("layout", a.OnLayout, a.layoutCompletions).
		LeafRestComplete("b", a.OnBuffer, a.bufferCompletions).
		LeafRestComplete("edit", a.OnEdit, a.editCompletions).
		Leaf("vs", a.SplitVertical).
		Leaf("split", a.SplitHorizontal).
		Leaf("clear", a.ClearFocus).
		Leaf("quit", a.Quit)
}

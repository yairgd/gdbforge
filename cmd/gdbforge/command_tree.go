package main

import (
	"github.com/yairgd/gdbforge/internal/commands"
)

// ExapData builds the command hierarchy on commandReg.Root:
//
//	/ → window → left, right, up, down
//	/ → gdb → run, start, continue, next, step, finish, nexti, stepi, interrupt
//	/ → gdb → break → file, delete
//	/ → gdb → info → registers, threads
//	/ → ! <cmdline>  (Vim-style :!bash / :!ls — ExecClient + ExecWidget)
//	/ → AI <question> / ai <question>  (in-app LLM on live GDB)
//	/ → set → … (colors, equalalways, inferior-tty, …)
//	/ → layout <name> [asm]  (default | panels | classic | wide; optional asm)
//	/ → b <name>   (switch buffer)
//	/ → lua <func> [args…]
//	/ → edit [name]
//	/ → help, vs, split, only, clear, close, quit
//	/ → :N / :0 (numeric goto — handled in handleUnknownCommand, not the trie)
func (a *DebuggerApp) ExapData() {
	a.commandReg.Root.
		Group("window",
			commands.Cmd("left", a.OnFocusLeft),
			commands.Cmd("right", a.OnFocusRight),
			commands.Cmd("up", a.OnFocusUp),
			commands.Cmd("down", a.OnFocusDown),
		).
		Group("gdb",
			commands.Cmd("run", a.GdbRun),
			commands.Cmd("start", a.GdbStart),
			commands.Cmd("continue", a.GdbContinue),
			commands.Cmd("next", a.GdbNext),
			commands.Cmd("step", a.GdbStep),
			commands.Cmd("finish", a.GdbFinish),
			commands.Cmd("nexti", a.GdbNexti),
			commands.Cmd("stepi", a.GdbStepi),
			commands.Cmd("interrupt", a.GdbInterrupt),
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
			commands.Cmd("breakmain", a.SetBreakMainOn),
			commands.Cmd("nobreakmain", a.SetBreakMainOff),
			commands.Cmd("gdblistenprint", a.SetGdbListenPrintOn),
			commands.Cmd("nogdblistenprint", a.SetGdbListenPrintOff),
			commands.Cmd("gdbtargetprint", a.SetGdbTargetPrintOn),
			commands.Cmd("nogdbtargetprint", a.SetGdbTargetPrintOff),
			commands.CmdRestComplete("inferior-tty", a.SetInferiorTTYCmd, a.inferiorTTYCompletions),
			commands.CmdRest("markcolor", a.SetMarkColor),
			commands.CmdRest("markdimcolor", a.SetMarkDimColor),
			commands.CmdRest("breakcolor", a.SetBreakColor),
			commands.CmdRest("breakdisabledcolor", a.SetBreakDisabledColor),
			commands.CmdRest("breakcondcolor", a.SetBreakCondColor),
			commands.CmdRest("pccolor", a.SetPCColor),
			commands.CmdRest("stackbreakcolor", a.SetStackBreakColor),
			commands.CmdRest("codeselcolor", a.SetCodeSelColor),
			commands.CmdRest("mutedcolor", a.SetMutedColor),
			commands.CmdRest("searchcolor", a.SetSearchColor),
		).
		LeafRestComplete("layout", a.OnLayout, a.layoutCompletions).
		LeafRestComplete("b", a.OnBuffer, a.bufferCompletions).
		LeafRestComplete("edit", a.OnEdit, a.editCompletions).
		LeafRestComplete("lua", a.OnLua, a.luaCompletions).
		Leaf("help", a.OnHelp).
		LeafRestComplete("vs", a.SplitVertical, a.splitAsmCompletions).
		LeafRestComplete("vsplit", a.SplitVertical, a.splitAsmCompletions).
		LeafRestComplete("sp", a.SplitHorizontal, a.splitAsmCompletions).
		LeafRestComplete("split", a.SplitHorizontal, a.splitAsmCompletions).
		Leaf("only", a.OnlyFocus).
		Leaf("clear", a.ClearFocus).
		Leaf("close", a.ClosePane).
		Leaf("quit", a.Quit)
}

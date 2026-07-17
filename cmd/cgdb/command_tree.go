package main

import (
	"github.com/yairgd/cgdb-go/internal/commands"
)

// ExapData builds the example command hierarchy on commandReg.Root:
//
//	/ → window → left, right, up, down
//	/ → window → break → file, delete
//	/ → break  → file, delete
//	/ → info   → registers, threads
//	/ → ! <cmdline>  (Vim-style :!bash / :!ls — ExecClient + ExecWidget)
//	/ → edit   → about, gdb, exec  (built-in views)
//	/ → vs, split, clear, quit
func (a *DebuggerApp) ExapData() {
	a.commandReg.Root.
		Group("window",
			commands.Cmd("left", a.OnFocusLeft),
			commands.Cmd("right", a.OnFocusRight),
			commands.Cmd("up", a.OnFocusUp),
			commands.Cmd("down", a.OnFocusDown),
			commands.Group("break",
				commands.Cmd("file", a.BreakFile),
				commands.Cmd("delete", a.DeleteBreakpoint),
			),
		).
		Group("break",
			commands.Cmd("file", a.BreakFile),
			commands.Cmd("delete", a.DeleteBreakpoint),
		).
		Group("info",
			commands.Cmd("registers", a.ShowRegisters),
			commands.Cmd("threads", a.ShowThreads),
		).
		LeafRest("!", a.OnRun).
		Group("edit",
			commands.Cmd("about", a.showBuiltin("about")),
			commands.Cmd("gdb", a.showBuiltin("gdb")),
			commands.Cmd("exec", a.showBuiltin("exec")),
		).
		Leaf("vs", a.SplitVertical).
		Leaf("split", a.SplitHorizontal).
		Leaf("clear", a.ClearFocus).
		Leaf("quit", a.Quit)
}

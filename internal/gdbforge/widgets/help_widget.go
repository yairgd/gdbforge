package widgets

import (
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// HelpWidget is a scrollable Viewport user manual (:help / :b help).
type HelpWidget struct {
	termui.BaseWidget
	viewport *termui.Viewport
	buf      *platform.Buffer
}

// NewHelpWidget caches the guide text into a read-only Viewport buffer.
func NewHelpWidget() *HelpWidget {
	buf := platform.NewBuffer()
	vp := termui.NewViewport(buf)
	vp.SetFollowTail(false)
	vp.SetReadOnly(true)
	vp.SetCursorVisible(false)
	vp.LineStyle = helpLineStyle

	w := &HelpWidget{
		BaseWidget: termui.BaseWidget{PaneName: "Help"},
		viewport:   vp,
		buf:        buf,
	}
	for _, line := range buildHelpLines() {
		buf.AppendLine(line)
	}
	vp.Home()
	w.initKeyBindings()
	return w
}

func helpLineStyle(line string) tcell.Style {
	st := tcell.StyleDefault
	if line == "gdbforge — user manual" {
		return st.Foreground(tcell.ColorYellow).Bold(true)
	}
	if strings.HasPrefix(line, "===") {
		return st.Foreground(tcell.ColorTeal).Bold(true)
	}
	if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, " ") && line != "" {
		return st.Foreground(tcell.ColorGray)
	}
	return st
}

// buildHelpLines is the in-app user manual (:help / :b help).
// Keep the root README.md "User guide" section aligned with this text.
func buildHelpLines() []string {
	return []string{
		"gdbforge — user manual",
		"",
		"Open this page with  :help  or  :b help",
		"Scroll with j/k, Up/Down, PgUp/PgDn, g/G, or the mouse wheel.",
		"This page is a read-only Viewport (same scroll/copy path as other panes).",
		"Press Ctrl-O to jump back to the previous pane.",
		"",
		"=== Overview ===",
		"",
		"gdbforge (Extreme Tooling Suite) is a Vim-inspired terminal debugger",
		"built on GDB. The screen is a multi-pane workspace: Code (or logo),",
		"GDB console, IO, Threads, Call Stack, Breakpoints, plus a global",
		": command line at the bottom.",
		"",
		"Status line colors:",
		"    Green  — insert mode (pane actively typing / focused for edit)",
		"    Blue   — normal mode (pane remembered / not insert-active)",
		"",
		"=== Modes ===",
		"",
		"Normal:",
		"    Default navigation mode. Global keys (n/s, Space, window focus)",
		"    and focused-pane keys apply. Esc leaves insert into normal.",
		"",
		"Insert:",
		"    Type into the focused console (usually GDB). Enter with i",
		"    (focuses GDB) or by clicking a pane. Esc returns to normal.",
		"    When GDB is focused, printable keys (including Space) go to GDB.",
		"",
		"Command:",
		"    Enter with : or by clicking the bottom cmdline. Type a command,",
		"    Tab for completion, Enter to run, Esc to cancel.",
		"",
		"Completion:",
		"    After Tab with multiple matches, the wildmenu opens above :.",
		"    Left/Right/Tab cycle; Enter accepts; Esc cancels back to command.",
		"",
		"=== Global keys (normal mode) ===",
		"",
		"    :              enter command mode",
		"    i              focus GDB leaf and enter insert",
		"    Esc            leave insert; focus last non-Code/non-GDB pane",
		"                   if one was active, else the Code/logo leaf",
		"                   (:set noesctocode keeps focus on current pane)",
		"    n              GDB next (also in insert when Code is focused)",
		"    s              GDB step (also in insert when Code is focused)",
		"    Up / Down      move Code cursor line (global, with fallthrough)",
		"    Space          toggle breakpoint: Call Stack selection, or",
		"                   Code cursor line (never steals Space from GDB)",
		"    e              enable/disable breakpoint at Code cursor",
		"    Ctrl-W h/j/k/l focus left / down / up / right",
		"    Ctrl-W arrows  same as hjkl",
		"    Ctrl-O         jump back after :b / :edit / :!",
		"    Ctrl-D         quit (TermApp)",
		"",
		"=== Colon commands ===",
		"",
		"Buffers and views:",
		"    :help              open this manual (same as :b help)",
		"    :b <name>          switch builtin or already-open file buffer",
		"    :b help|about|gdb|io|output|breakpoint|threads|callstack|logger|exec",
		"    :edit              project source file picker",
		"    :edit <file>       open that source in a CodeWidget",
		"    :e                 unique prefix of :edit",
		"",
		"Layout:",
		"    :layout panels     startup layout (Code|GDB left; IO +",
		"                       Threads|Callstack over Breakpoints)",
		"    :layout default    six-pane workspace",
		"    :layout classic    full-width Code over GDB",
		"    :layout            re-apply panels",
		"    :vs / :split       vertical / horizontal split",
		"",
		"Settings (:set …):",
		"    equalalways / noequalalways",
		"    clearoutput / noclearoutput",
		"        Clear IO pane when GDB session starts (default on).",
		"    continueafterclear / nocontinueafterclear",
		"        After removing a BP while running, auto-continue (default off).",
		"        Inserting a BP while running still auto-continues.",
		"        frame/thread commands never auto-continue.",
		"    esctocode / noesctocode",
		"        Esc restores last pane / Code (default on).",
		"    breakmain / nobreakmain",
		"        Insert break main on GDB start (default on).",
		"    gdblistenprint / nogdblistenprint",
		"        Paint App/MCP replies in GDB console (default on).",
		"    markcolor <name>           focused list selection (default blue)",
		"    markdimcolor <name>        unfocused selection (default gray)",
		"    breakcolor <name>          enabled BP bg (default red)",
		"    breakdisabledcolor <name>  disabled BP bg (default yellow)",
		"",
		"Other:",
		"    :! <cmd>           run shell in an Exec pane",
		"    :AI <question>     in-app LLM against live GDB (:ai alias)",
		"    :clear             clear focused clearable pane",
		"    :quit              close focused pane / quit app",
		"    :window left|right|up|down   focus direction",
		"    :gdb break file / :gdb break delete",
		"    :gdb info registers|threads",
		"",
		"=== Per-pane reference ===",
		"",
		"Code (or startup logo):",
		"    Startup shows the gdbforge logo until a source file is opened",
		"    (*stopped, :edit, or file picker). Then Logo is replaced by Code.",
		"    Up/Down or j/k   bold cursor line",
		"    Space            insert/remove breakpoint at cursor",
		"    e                enable/disable BP (yellow gutter when disabled)",
		"    Missing/.so src  centered \"not available\" + path",
		"    Status line      full file path when focused",
		"",
		"GDB console:",
		"    Insert mode to type CLI; Enter submits; Ctrl-C interrupt;",
		"    Ctrl-D sends q; Ctrl-L clear; Ctrl-V / middle-click paste.",
		"    frame / f / up / down sync Code + Call Stack after (gdb) prompt.",
		"    Mouse drag selects scrollback; Ctrl-C copies selection.",
		"",
		"Breakpoints (:b breakpoint):",
		"    j/k or Up/Down / Enter / click — select and show source",
		"    e — toggle enable (disabled rows stay in list, removed from GDB)",
		"    d — delete from list and GDB",
		"    Row colors from breakcolor / breakdisabledcolor",
		"",
		"Threads (:b threads):",
		"    j/k Up/Down Enter click wheel — select thread, send thread <id>,",
		"    refresh stack, show current frame source",
		"",
		"Call Stack (:b callstack):",
		"    j/k Up/Down Enter click wheel — select frame, send frame N,",
		"    show source (or not-available placeholder)",
		"    Space — toggle breakpoint at selected frame location",
		"",
		"IO console (:b io, alias :b output):",
		"    Program stdin/stdout (dedicated PTY). Type here while running;",
		"    Enter sends to the inferior. PgUp/PgDn scroll; Ctrl-L clear;",
		"    Ctrl-C / Ctrl-D → program interrupt / EOF. ANSI colors.",
		"    GDB console keys never go to the program.",
		"",
		"About (:b about):",
		"    Version, build info, author, license (credits live here only)",
		"",
		"=== Clipboard and mouse ===",
		"",
		"    Ctrl-C / Ctrl-X   copy / cut selection (viewports) or cmdline text",
		"    Ctrl-V            paste into console input or : cmdline",
		"    Middle-click      paste (Linux terminal style)",
		"    Click : line      enter command mode; caret follows column",
		"    Click outside :   leave command mode, then focus that pane",
		"    Drag in scrollback  select text; release copies",
		"",
		"=== Breakpoints while the inferior is running ===",
		"",
		"    Space / BP pane may interrupt GDB (Ctrl-C), send break/clear,",
		"    then continue only for insert (or clear if continueafterclear).",
		"    Switching threads/frames while running interrupts but does not",
		"    auto-continue.",
		"",
		"=== Tips ===",
		"",
		"    Tab after :b or :edit lists candidates in the wildmenu.",
		"    Use :layout panels if the workspace looks wrong after splits.",
		"    Prefer :edit for project files; :b is for open buffers + builtins.",
		"",
		"=== See also ===",
		"",
		"    :b about     credits and version",
		"    :help        this manual",
	}
}

func (w *HelpWidget) initKeyBindings() {
	w.BindKeyFunc("scroll-up", func(args ...any) { w.viewport.ScrollLineUp() }, "<Up>", "k")
	w.BindKeyFunc("scroll-down", func(args ...any) { w.viewport.ScrollLineDown() }, "<Down>", "j")
	w.BindKeyFunc("scroll-left", func(args ...any) { w.viewport.ViewScrollColLeft() }, "<Left>")
	w.BindKeyFunc("scroll-right", func(args ...any) { w.viewport.ViewScrollColRight() }, "<Right>")
	w.BindKeyFunc("page-up", func(args ...any) { w.viewport.ScrollPageUp(10) }, "<PgUp>", "<C-b>")
	w.BindKeyFunc("page-down", func(args ...any) { w.viewport.ScrollPageDown(10) }, "<PgDn>", "<C-f>")
	w.BindKeyFunc("home", func(args ...any) { w.viewport.ScrollHome() }, "<Home>", "g")
	w.BindKeyFunc("end", func(args ...any) { w.viewport.ScrollEnd() }, "<End>", "G")
}

func (w *HelpWidget) HandleFocusKey(ev *tcell.EventKey) bool {
	return w.HandleBoundKey(ev)
}

func (w *HelpWidget) HandleEvent(ev tcell.Event) {
	switch e := ev.(type) {
	case *tcell.EventMouse:
		w.viewport.HandleEvent(e)
	case *tcell.EventKey:
		if w.HandleBoundKey(e) {
			return
		}
		w.viewport.HandleEvent(e)
	}
}

func (w *HelpWidget) SetFocused(focused bool) {
	w.BaseWidget.SetFocused(focused)
	w.viewport.SetCursorVisible(false)
}

func (w *HelpWidget) SetClipboard(io termui.ClipboardIO) {
	w.viewport.SetClipboard(io)
}

func (w *HelpWidget) Draw(c termui.Canvas) {
	w.viewport.Draw(c)
}

func (w *HelpWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

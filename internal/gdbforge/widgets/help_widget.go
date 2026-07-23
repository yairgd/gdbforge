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
		"    Default navigation mode. Global keys (n/s/c, Space, window focus)",
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
		"    Left/Right/Tab cycle selection only (does not edit the line).",
		"    Typed keys edit the source line (cmdline or GDB); the list re-queries.",
		"    Enter accepts; Esc cancels.",
		"    The bar is only a view — a list window could replace it later.",
		"",
		"Lua (ModeLua):",
		"    Enter via :b snake, :b tetris, or :b lua. All keys go to the",
		"    Lua pane until Esc returns to normal mode.",
		"",
		"=== Global keys (any mode) ===",
		"",
		"    Ctrl-Z         suspend inferior if running, else suspend gdbforge",
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
		"    c              GDB continue (also in insert when Code is focused)",
		"    Up / Down      move Code cursor line (global, with fallthrough)",
		"    Space          toggle breakpoint: Call Stack selection, or",
		"                   Code cursor line (never steals Space from GDB)",
		"    e              enable/disable breakpoint at Code cursor",
		"    Ctrl-W h/j/k/l focus left / down / up / right",
		"    Ctrl-W arrows  same as hjkl",
		"    Ctrl-O         jump back after :b / :edit / :!",
		"    Ctrl-D         send q to GDB (confirm if inferior alive)",
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
		"    :layout wide       startup layout (Code|IO over",
		"                       GDB|(Threads|Callstack / Breakpoints))",
		"    :layout panels     Code|GDB left; IO + Threads|Callstack over BP",
		"    :layout default    six-pane workspace",
		"    :layout classic    full-width Code over GDB",
		"    :layout            re-apply wide",
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
		"        Skipped when restoring .gdbforge/breakpoints.yaml or using -x.",
		"    gdblistenprint / nogdblistenprint",
		"        Paint App/MCP replies in GDB console (default on).",
		"    gdbtargetprint / nogdbtargetprint",
		"        Also paint program stdout in GDB console (default off),",
		"        like a classic shared GDB terminal. IO pane always uses",
		"        the inferior PTY; this only mirrors into GDB.",
		"    markcolor <name>           focused list selection (default blue)",
		"    markdimcolor <name>        unfocused selection (default gray)",
		"    breakcolor <name>          enabled BP bg (default red)",
		"    breakdisabledcolor <name>  disabled BP bg (default yellow)",
		"    pccolor <name>             Code ━━▶ row bg (default darkslategray)",
		"    stackbreakcolor <name>     green mark at stop PC (default green)",
		"        Breakpoints / Call Stack #0 / current Thread when StopLocation",
		"        matches that row (━━▶ / real PC). Selected BP at stop stays green.",
		"    codeselcolor <name>        Code browse cursor bg (default darkblue)",
		"    mutedcolor <name>          empty-list / dim text (default gray)",
		"",
		"Other:",
		"    :! <cmd>           run shell in an Exec pane",
		"    :AI <question>     in-app LLM against live GDB (:ai alias)",
		"    :lua <func> [args] gdbforge Lua command (e.g. :lua hello)",
		"    :clear             clear focused clearable pane",
		"    :quit              close focused pane / quit app",
		"    :window left|right|up|down   focus direction",
		"    :gdb break file / :gdb break delete",
		"    :gdb info registers|threads",
		"",
		"=== Lua / gdbforge ===",
		"",
		"    :b snake / :b tetris   playable demos (arrows; r restart)",
		"    :b lua                 scratch pane; keys echo via on_key",
		"    :b exec                optional Exec logs (after gdbforge.spawn)",
		"    :b code / :b gdb       focus Code / GDB leaf (does not steal panes)",
		"    User scripts: ./.gdbforge/lua/<name>.lua → :lua <name>",
		"    gdbforge.print(...)    append line to the Lua pane",
		"    gdbforge.register(name, fn)  expose fn to :lua name",
		"    gdbforge.spawn(...)    background PTY (JLink); no focus steal",
		"    gdbforge.run(...)      interactive :! (replaces focused pane)",
		"    gdbforge.gdb(cmd)      send CLI to GDB console",
		"    gdbforge.open_buffer(\"code\"|\"gdb\"|…)  leaf-aware focus",
		"    pane.set_cell / pane.clear / pane.size   full-widget draw API",
		"",
		"=== Per-pane reference ===",
		"",
		"Code (or startup logo):",
		"    Startup shows the gdbforge logo until a source file is opened",
		"    (*stopped, :edit, or file picker). Then Logo is replaced by Code.",
		"    ━━▶              real program counter (StopLocation from *stopped)",
		"    Blue cursor line browse selection (j/k, or Jump from Breakpoints)",
		"    Up/Down or j/k   move blue cursor (does not move ━━▶)",
		"    Space            insert/remove breakpoint at cursor",
		"    e                enable/disable BP (yellow gutter when disabled)",
		"    Missing/.so src  centered \"not available\" + path",
		"    Status line      full file path when focused",
		"",
		"GDB console:",
		"    Insert mode to type CLI; Enter submits; Tab completes (wildmenu);",
		"    Unique Tab completion appends a trailing space (e.g. ju → jump ).",
		"    Enter on empty line repeats the last command;",
		"    n/s keys and typed next/step/continue use MI -exec-* (no CLI",
		"    source dump — Code pane follows *stopped);",
		"    q/quit with a live inferior asks Quit anyway? (MI has no confirm);",
		"    Quit anyway?: y / n / Enter (=n);",
		"    Ctrl-C interrupt (MI Quit / SIGINT);",
		"    Ctrl-Z suspend inferior or gdbforge (any mode / any focused pane);",
		"    Ctrl-D sends q (app exits when GDB ends);",
		"    Ctrl-L clear; Ctrl-V / middle-click paste.",
		"    frame / f / up / down sync Code + Call Stack after (gdb) prompt.",
		"    Mouse drag selects scrollback; double-click selects a word (copies);",
		"    triple-click selects the line; Ctrl-C copies selection.",
		"    :set gdbtargetprint also shows program stdout here (legacy).",
		"",
		"Breakpoints (:b breakpoint):",
		"    j/k or Up/Down / Enter / click — select; Code jumps with blue cursor",
		"    (━━▶ stays on real PC). Selected row at stop PC stays green.",
		"    e — toggle enable (disabled rows stay in list, removed from GDB)",
		"    d — delete from list and GDB",
		"    Row colors: breakcolor / breakdisabledcolor; green = stop PC",
		"    Persist: saved to ./.gdbforge/breakpoints.yaml on quit (q / Ctrl-D);",
		"    restored on next start from the same cwd (usually the build dir).",
		"",
		"Threads (:b threads):",
		"    j/k Up/Down Enter click wheel — select thread (-thread-select),",
		"    refresh stack, show current frame source",
		"    Green row: current thread whose location matches stop PC (━━▶)",
		"",
		"Call Stack (:b callstack):",
		"    j/k Up/Down Enter click wheel — select frame (-stack-select-frame),",
		"    show source (or not-available placeholder); no CLI frame dump",
		"    Space — toggle breakpoint at selected frame location",
		"    Green row: frame 0 only, when it matches stop PC (━━▶)",
		"",
		"IO console (:b io, alias :b output):",
		"    Program stdin/stdout from the inferior PTY only (not MI @\").",
		"    Type here while running; Enter sends to the inferior.",
		"    PgUp/PgDn scroll; Ctrl-L clear; Ctrl-C / Ctrl-D → interrupt / EOF.",
		"    GDB console keys never go to the program.",
		"",
		"About (:b about):",
		"    Version, build info, author, license (credits live here only)",
		"",
		"=== Session / GDB startup ===",
		"",
		"    gdbforge [opts] [--] [gdb opts]   e.g. -- -nx -x script.gdb elf",
		"    Waits for first (gdb) before app MI (so -x remote/load can finish).",
		"    Injects: set pagination off (no --Type <RET> during load).",
		"    Breakpoints YAML: ./.gdbforge/breakpoints.yaml (cwd).",
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
		"    Use :layout wide if the workspace looks wrong after splits.",
		"    Prefer :edit for project files; :b is for open buffers + builtins.",
		"    Run gdbforge from the build dir so .gdbforge/breakpoints.yaml matches.",
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

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
// Full markdown twin: docs/USER_GUIDE.md. Lua reference: docs/LUA_API.md.
func buildHelpLines() []string {
	return []string{
		"gdbforge — user manual",
		"",
		"Open this page with  :help  or  :b help",
		"Scroll with j/k, Up/Down, PgUp/PgDn, g/G, or the mouse wheel.",
		"This page is a read-only Viewport (same scroll/copy path as other panes).",
		"Press Ctrl-O to jump back to the previous pane.",
		"",
		"Full manual (GitHub / ./docs/serve.sh):  docs/USER_GUIDE.md",
		"Lua API reference:                       docs/LUA_API.md",
		"Script catalog (ships in binary; override via .gdbforge/lua): lua/README.md",
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
		"    Enter via :lua snake / :lua tetris, or :b lua. All keys go to the",
		"    Lua pane until Esc returns to normal mode.",
		"",
		"=== Global keys (any mode) ===",
		"",
		"    Ctrl-Z         suspend inferior if running, else suspend gdbforge",
		"    Ctrl-C         interrupt inferior / debugger (GDB/dlv ^C)",
		"    Ctrl-D         send q / quit (confirm if inferior alive);",
		"                   app exits when the debugger session ends",
		"",
		"=== Global keys (normal mode) ===",
		"",
		"    :              enter command mode",
		"    /              search in focused pane (live highlight; Enter commits)",
		"    * / #          next / previous search match (n stays GDB next)",
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
		"    Ctrl-W o       only — close other panes, keep focused",
		"    Ctrl-W Ctrl-O  same as Ctrl-W o",
		"    Ctrl-O         jump back after :b / :edit / :!",
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
		"    :only              keep focused pane only (Ctrl-W o)",
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
		"    inferior-tty [internal|/dev/pts/N]",
		"        No arg: open an external terminal and route program stdio",
		"        there (TUI apps and high-rate stdout — :b io is a line",
		"        console; flood paint is less smooth than a real terminal).",
		"        internal: restore the in-app IO pane.",
		"        Optional /dev/pts/N: use an already-open slave (advanced).",
		"        Tab: internal.",
		"        Env: GDBFORGE_INFERIOR_TTY=/dev/pts/N at startup;",
		"             GDBFORGE_TERMINAL=kitty|xterm|mate-terminal|…",
		"        GDB: live -inferior-tty-set (no session restart).",
		"        Delve (-g dlv): restarts dlv exec with --tty <path>",
		"        (same program args). Prefer :lua dlv_port for Go TUIs",
		"        (headless dlv in the other window + dlv connect).",
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
		"    searchcolor <name>         /search match bg (default darkorange)",
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
		"    :lua snake / tetris     demos (then :b name to refocus; r restart)",
		"    :b lua                 scratch pane; keys echo via on_key",
		"    :b exec                optional Exec logs (after gdbforge.spawn)",
		"    :b code / :b gdb       focus Code / GDB leaf (does not steal panes)",
		"    User scripts (first basename wins):",
		"      1) ./.gdbforge/lua/**/*.lua",
		"      2) ~/.gdbforge/lua/**/*.lua",
		"      3) embedded catalog (ships with binary)",
		"    → :lua <basename>  (nested dirs OK — see lua/README.md).",
		"    Full gdbforge.* docs: docs/LUA_API.md",
		"    gdbforge.print(...)    append line to the Lua pane",
		"    gdbforge.register(name, fn)  expose fn to :lua name",
		"    gdbforge.spawn(...)    background PTY (JLink); no focus steal",
		"    gdbforge.spawn_terminal(...)  real terminal + argv (gdbserver/TUI)",
		"    gdbforge.open_external_tty()  hold pts in kitty/xterm; return path",
		"    gdbforge.set_inferior_tty(path|\"internal\")",
		"        GDB: live -inferior-tty-set. Delve: restart with --tty.",
		"    gdbforge.dlv_connect(addr)   replace session with dlv connect",
		"    gdbforge.spawn_dlv_headless(port [, args…])  headless dlv in a",
		"        real terminal (stdio stays there; then dlv_connect).",
		"    gdbforge.program()      debuggee path from gdbforge startup args",
		"    gdbforge.run(...)      interactive :! (replaces focused pane)",
		"    gdbforge.gdb(cmd)      send CLI to GDB/dlv console",
		"    gdbforge.open_buffer(\"code\"|\"gdb\"|…)  leaf-aware focus",
		"    pane.set_cell / pane.clear / pane.size   full-widget draw API",
		"    TUI debug (GDB): :lua terminal_debug [prog] [run]",
		"    Go + Delve external port: :lua dlv_ext_port [port] [extra args…]",
		"        (alias :lua dlv_port) — headless dlv + connect",
		"        Example: gdbforge -g dlv ./hello-go → :lua dlv_ext_port 1234",
		"    Embedded Linux board: :lua remotegdb [app] [host] [port]",
		"        scp if MD5 changed → ssh gdbserver → target remote",
		"        Env: GDBFORGE_REMOTE_APP/HOST/USER/PORT/DIR",
		"    Terminal emulator: export GDBFORGE_TERMINAL=mate-terminal",
		"        (or kitty|xterm|gnome-terminal|…). Catalog: lua/README.md",
		"",
		"=== External terminal (program stdio) ===",
		"",
		"    Why: :b io is a line console, not a full VT. Use an external",
		"    terminal for TUI/curses apps AND for high-rate stdout.",
		"    Under a printf flood, :b io stays interruptible (Ctrl-C) but",
		"    paint is less smooth than mate-terminal — known GUI limit.",
		"",
		"    Advantages of :set inferior-tty:",
		"      - Smooth scroll/paint (emulator owns the PTY master)",
		"      - Real VT (curses, menus, alternate screen)",
		"      - Program I/O does not compete with gdbforge redraw",
		"      - GDB: live -inferior-tty-set (no session restart)",
		"",
		"    Quick switch (both backends):",
		"      :set inferior-tty           open terminal + route stdio there",
		"      :set inferior-tty internal  back to :b io",
		"      :lua external_tty           same via Lua (GDB)",
		"      export GDBFORGE_TERMINAL=mate-terminal   # before starting gdbforge",
		"",
		"    GDB vs Delve:",
		"      GDB  — -inferior-tty-set applies live; session stays up.",
		"             Also: :lua terminal_debug / gdbserver_tui / remotegdb.",
		"      Delve — --tty is only at spawn time; :set inferior-tty",
		"             restarts `dlv exec` (BPs re-applied when possible).",
		"             Preferred Go flow (:lua dlv_ext_port / dlv_port):",
		"               1) ~/gdbforge/bin/gdbforge -g dlv ./hello-go",
		"               2) :lua dlv_ext_port 1234   # optional: extra prog args",
		"               3) headless dlv opens in another window;",
		"                  this session becomes `dlv connect` —",
		"                  inferior stdin/stdout stay in that window.",
		"             Full recipes + env vars: lua/README.md",
		"",
		"    Closing the external window does not auto-rewire IO;",
		"    use :set inferior-tty internal when done.",
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
		"GDB / Delve console:",
		"    Insert mode to type CLI; Enter submits; Tab completes (wildmenu);",
		"    Unique Tab completion appends a trailing space (e.g. ju → jump ).",
		"    Tab (GDB): MI -complete (commands, files, symbols).",
		"    Tab (Delve -g dlv): command names; for b/break/trace/… locspecs",
		"        runs funcs ^<prefix> (e.g. b main. → main.main).",
		"        File:line locspecs are not completed yet.",
		"    Enter on empty line repeats the last command;",
		"    n/s/c keys and typed next/step/continue use MI -exec-* under GDB",
		"    (no CLI source dump — Code pane follows *stopped);",
		"    under Delve they are plain CLI;",
		"    q/quit with a live inferior asks Quit anyway? (MI has no confirm);",
		"    Quit anyway?: y / n / Enter (=n);",
		"    After exit, Delve may ask Set a suspended breakpoint … [Y/n]?;",
		"        answer y/n on the live host (Ctrl-C sends n).",
		"    Ctrl-C interrupt (MI Quit / SIGINT); under Delve only when the",
		"        inferior is running (not at [Y/n]?);",
		"    Ctrl-Z suspend inferior or gdbforge (any mode / any focused pane);",
		"    Ctrl-C interrupt inferior via debugger (any mode / any pane);",
		"    Ctrl-D send q / quit (any mode / any focused pane);",
		"    app exits when the debugger session ends;",
		"    Ctrl-L clear; Ctrl-V / middle-click paste.",
		"    frame / f / up / down sync Code + Call Stack after (gdb)/(dlv).",
		"    Mouse drag selects scrollback; double-click selects a word (copies);",
		"    triple-click selects the line; Ctrl-C copies selection.",
		"    :set gdbtargetprint also shows program stdout here (legacy).",
		"",
		"Examples:",
		"    Screencasts: docs/media/gdbforge-demo-r5.mp4,",
		"                 docs/media/gdbforge-demo-linux-app.mp4",
		"    Demo target: Cortex-R5 on MPSoC via SEGGER J-Link;",
		"      Lua bring-up: lua/r5_debug (:lua r5_debug)",
		"      (spawn JLinkGDBServer, wait_port, target remote, load).",
		"    Deep stack sample: examples/stack_demo.c",
		"      gcc -g -O0 -o stack_demo examples/stack_demo.c",
		"      ./bin/gdbforge ./stack_demo",
		"      break leaf; run; :b callstack — or n/s/c from Code.",
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
		"    Default: program stdin/stdout on an internal PTY (not MI @\").",
		"    Type here while running; Enter sends to the inferior.",
		"    PgUp/PgDn scroll; Ctrl-L clear; Ctrl-C → interrupt.",
		"    Global Ctrl-D quits the debugger (not program EOF).",
		"    GDB console keys never go to the program.",
		"    External / TUI: :set inferior-tty (see \"External terminal\" above).",
		"      When external, this pane shows a note only — type in the",
		"      other window. GDB switches live; Delve restarts with --tty",
		"      (or use :lua dlv_port so stdio never leaves the other window).",
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
		"    :help        this manual (short)",
		"    docs/USER_GUIDE.md   full user manual (markdown)",
		"    docs/LUA_API.md      Lua / gdbforge.* reference",
		"    lua/README.md        installable workflow scripts",
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

func (w *HelpWidget) Viewport() *termui.Viewport {
	return w.viewport
}

func (w *HelpWidget) LinesForTest() []string {
	n := w.buf.NumLines()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = w.buf.Line(i)
	}
	return out
}

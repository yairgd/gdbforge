# User guide

In-app twin: **`:help`** / **`:b help`** (source: `buildHelpLines()` in [`internal/gdbforge/widgets/help_widget.go`](../internal/gdbforge/widgets/help_widget.go)).

Browse this file on GitHub or via `./docs/serve.sh`. For Lua scripting details see [LUA_API.md](LUA_API.md). Script catalog: [../lua/README.md](../lua/README.md).

---

### Overview

gdbforge (gdbforge: Extreme Tooling Suite) is a Vim-inspired terminal debugger built on GDB. The screen is a multi-pane workspace: Code (or logo), GDB console, IO, Threads, Call Stack, Breakpoints, plus a global `:` command line at the bottom.

**Status line colors:**

- Green — insert mode (pane actively typing / focused for edit)
- Blue — normal mode (pane remembered / not insert-active)

### Modes

**Normal**

Default navigation mode. Global keys (`n`/`s`/`c`, Space, window focus) and focused-pane keys apply. Esc leaves insert into normal.

**Insert**

Type into the focused console (usually GDB). Enter with `i` (focuses GDB) or by clicking a pane. Esc returns to normal. When GDB is focused, printable keys (including Space) go to GDB.

**Command**

Enter with `:` or by clicking the bottom cmdline. Type a command, Tab for completion, Enter to run, Esc to cancel.

**Completion**

After Tab with multiple matches, the wildmenu opens above `:`. Left/Right/Tab cycle; type letters to narrow (CompletionMenu); Enter accepts; Esc cancels. The completion bar is a replaceable view over that menu.

### Global keys (any mode)

| Key | Action |
|-----|--------|
| Ctrl-Z | suspend inferior if running, else suspend gdbforge |
| Ctrl-C | interrupt inferior / debugger (same as GDB console Ctrl-C) |
| Ctrl-D | send `q` / quit (confirm if inferior alive); app exits when the debugger session ends |

### Global keys (normal mode)

| Key | Action |
|-----|--------|
| `:` | enter command mode |
| `i` | focus GDB leaf and enter insert |
| `Esc` | leave insert; focus last non-Code/non-GDB pane if one was active, else the Code/logo leaf (`:set noesctocode` keeps focus on current pane) |
| `n` | GDB next via MI `-exec-next` (also in insert when Code is focused) |
| `s` | GDB step via MI `-exec-step` (also in insert when Code is focused) |
| `c` | GDB continue via MI `-exec-continue` (also in insert when Code is focused) |
| Up / Down | move Code browse cursor (does not move ━━▶) |
| Space | toggle breakpoint: Call Stack selection, or Code cursor line (never steals Space from GDB) |
| `e` | enable/disable breakpoint at Code cursor |
| Ctrl-W h/j/k/l | focus left / down / up / right |
| Ctrl-W arrows | same as hjkl |
| Ctrl-O | jump back after `:b` / `:edit` / `:!` |

### Colon commands

**Buffers and views**

- `:help` — open this manual (same as `:b help`)
- `:b <name>` — switch builtin or already-open file buffer
- `:b help|about|gdb|output|breakpoint|threads|callstack|logger|exec`
- `:edit` — project source file picker
- `:edit <file>` — open that source in a CodeWidget
- `:e` — unique prefix of `:edit`

**Layout**

- `:layout wide` — startup layout (Code|IO over GDB|(Threads|Callstack / Breakpoints))
- `:layout panels` — Code|GDB left; IO + Threads|Callstack over Breakpoints
- `:layout default` — six-pane workspace
- `:layout classic` — full-width Code over GDB
- `:layout` — re-apply wide
- `:vs` / `:split` — vertical / horizontal split

**Settings (`:set …`)**

- `equalalways` / `noequalalways`
- `clearoutput` / `noclearoutput` — clear IO pane when GDB session starts (default on)
- `continueafterclear` / `nocontinueafterclear` — after removing a BP while running, auto-continue (default off). Inserting a BP while running still auto-continues. `frame`/`thread` commands never auto-continue.
- `esctocode` / `noesctocode` — Esc restores last pane / Code (default on)
- `breakmain` / `nobreakmain` — insert `break main` on GDB start (default on); skipped when restoring `./.gdbforge/breakpoints.yaml` or when using `-x`/`-ex`
- `gdblistenprint` / `nogdblistenprint` — paint App/MCP replies in GDB console (default on)
- `gdbtargetprint` / `nogdbtargetprint` — also paint program stdout in GDB console like a classic terminal (default off); IO pane always uses the inferior PTY
- `inferior-tty` / `inferior-tty internal` — route program stdio to an **external terminal** (TUI apps need a real VT; `:b io` is only a line console):
  - bare `:set inferior-tty` — open a terminal (`GDBFORGE_TERMINAL`) and attach stdio there
  - `:set inferior-tty internal` — restore the in-app IO pane
  - optional `/dev/pts/N` — use an already-open slave; startup: `GDBFORGE_INFERIOR_TTY`
  - **GDB:** live `-inferior-tty-set` (no restart)
  - **Delve (`-g dlv`):** restarts `dlv exec --tty …` (same program args). For Go TUIs prefer `:lua dlv_port` (headless dlv in another window + `dlv connect` — stdio stays there)
  - Details: [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#external-terminal-stdio-tui-targets)
- `markcolor <name>` — focused list selection (default blue)
- `markdimcolor <name>` — unfocused selection (default gray)
- `breakcolor <name>` — enabled BP bg (default red)
- `breakdisabledcolor <name>` — disabled BP bg (default yellow)
- `pccolor <name>` — Code ━━▶ row bg (default darkslategray)
- `stackbreakcolor <name>` — green mark at stop PC on Breakpoints / Call Stack #0 / current Thread (default green)
- `codeselcolor <name>` — Code browse cursor bg (default darkblue)
- `mutedcolor <name>` — empty-list / dim text (default gray)

**Other**

- `:! <cmd>` — run shell in an Exec pane
- `:AI <question>` — in-app LLM against live GDB (`:ai` alias)
- `:clear` — clear focused clearable pane
- `:quit` — close focused pane / quit app
- `:window left|right|up|down` — focus direction
- `:gdb break file` / `:gdb break delete`
- `:gdb info registers|threads`

### Per-pane reference

**Code (or startup logo)**

- Startup shows the gdbforge logo until a source file is opened (`*stopped`, `:edit`, or file picker). Then Logo is replaced by Code.
- ━━▶ — real program counter (`StopLocation` from `*stopped`)
- Blue cursor line — browse selection (j/k, or jump from Breakpoints); does not move ━━▶
- Space — insert/remove breakpoint at cursor
- `e` — enable/disable BP (yellow gutter when disabled)
- Missing/.so src — centered "not available" + path
- Status line — full file path when focused

**GDB / Delve console**

- Insert mode to type CLI; Enter submits; Tab completes (wildmenu); unique Tab completion appends a trailing space (e.g. `ju` → `jump `)
- **Tab (GDB):** MI `-complete` (commands, files, symbols)
- **Tab (Delve `-g dlv`):** command names from a static list; for `b` / `break` / `trace` / … locspecs, runs `funcs ^<prefix>` (e.g. `b main.` → `main.main`). File:`line` locspecs are not completed yet
- Enter on empty line repeats the last command
- `n`/`s`/`c` keys and typed `next`/`step`/`continue` use MI `-exec-*` under GDB (no CLI source dump — Code pane follows `*stopped`); under Delve they are plain CLI
- Ctrl-C interrupt (only when the inferior is running under Delve); Ctrl-Z suspend / Ctrl-D quit (any mode); Ctrl-L clear; Ctrl-V / middle-click paste
- After exit, Delve may ask `Set a suspended breakpoint … [Y/n]?` — answer `y`/`n` on the live host (Ctrl-C sends `n`)
- `frame` / `f` / `up` / `down` sync Code + Call Stack after `(gdb)` / `(dlv)` prompt
- Mouse drag selects scrollback; double-click word / triple-click line; Ctrl-C copies selection

**Breakpoints (`:b breakpoint`)**

- j/k or Up/Down / Enter / click-release — select; Code jumps with blue cursor (━━▶ stays on real PC). Selected row at stop PC stays green
- `e` — toggle enable (disabled rows stay in list, removed from GDB)
- `d` — delete from list and GDB
- Row colors: `breakcolor` / `breakdisabledcolor`; green = stop PC (`stackbreakcolor`)
- Persist: saved to `./.gdbforge/breakpoints.yaml` on quit (`q` / Ctrl-D); restored on next start from the same cwd

**Threads (`:b threads`)**

- j/k Up/Down Enter click-release — select thread (`-thread-select`), refresh stack, show current frame source
- Green row: current thread whose location matches stop PC (━━▶)

**Call Stack (`:b callstack`)**

- j/k Up/Down Enter click-release — select frame (`-stack-select-frame`), show source (or not-available placeholder); no CLI frame dump
- Space — toggle breakpoint at selected frame location
- Green row: frame 0 only, when it matches stop PC (━━▶)

**IO console (`:b io`, alias `:b output`)**

- Default: program stdin/stdout on a **dedicated PTY** (GDB: `-inferior-tty-set`; Delve: `dlv exec --tty`); debugger console uses a separate PTY
- Type here while the inferior is running; Enter sends to the program
- PgUp/PgDn scroll; Ctrl-L clear; Ctrl-C → program interrupt; ANSI colors
- **Not a VT emulator** — for TUI/curses targets use an external terminal:

| | GDB | Delve (`-g dlv`) |
|--|-----|------------------|
| `:set inferior-tty` | Live `-inferior-tty-set` | Restarts session with `--tty` |
| Recommended TUI flow | `:set inferior-tty` or `:lua terminal_debug` / `gdbserver_tui` | `:lua dlv_port [port]` — headless dlv in another window, then `dlv connect`; inferior I/O stays in that window |
| Env | `GDBFORGE_INFERIOR_TTY`, `GDBFORGE_TERMINAL` | same |

- When external, `:b io` shows a note only (type in the other window). Closing that window does not auto-rewire — `:set inferior-tty internal`
- Global Ctrl-D quits the debugger (same as GDB pane); it is not sent as program EOF

### External terminal (quick start)

```bash
# GDB — TUI target in another window
./bin/gdbforge ./my_tui
# then:  :set inferior-tty
# or:    :lua terminal_debug

# Delve — Go TUI (preferred: headless + connect)
./bin/gdbforge -g dlv -- ./my_go_tui
# then:  :lua dlv_port          # optional: :lua dlv_port 2345
# inferior stdin/stdout → the new terminal; this UI talks via dlv connect
```

More plugins: [`../lua/README.md`](../lua/README.md) (copy into `./.gdbforge/lua/`).

**About (`:b about`)**

- Version, build info, and license

### Session / GDB startup

- `gdbforge [opts] [--] [gdb opts]` — e.g. `-- -nx -x script.gdb elf`
- Waits for first `(gdb)` before app MI (so `-x` remote/load can finish)
- Injects `set pagination off` (no `--Type <RET>` during load)
- Breakpoints YAML: `./.gdbforge/breakpoints.yaml` (cwd — usually the build dir)

### Clipboard and mouse

- Ctrl-C / Ctrl-X — copy / cut selection (viewports) or cmdline text
- Ctrl-V — paste into console input or `:` cmdline
- Middle-click — paste (Linux terminal style)
- Click `:` line — enter command mode; caret follows column
- Click outside `:` — leave command mode, then focus that pane
- Drag in scrollback — select text; release copies

### Breakpoints while the inferior is running

Space / BP pane may interrupt GDB (Ctrl-C), send break/clear, then continue only for insert (or clear if `continueafterclear`). Switching threads/frames while running interrupts but does not auto-continue.

### Tips

- Tab after `:b` or `:edit` lists candidates in the wildmenu
- Use `:layout wide` if the workspace looks wrong after splits
- Prefer `:edit` for project files; `:b` is for open buffers + builtins
- Run gdbforge from the build dir so `./.gdbforge/breakpoints.yaml` matches your project

### See also

- `:b about` — version and license
- `:help` — in-app summary of this guide
- [LUA_API.md](LUA_API.md) — Lua / `gdbforge.*` reference

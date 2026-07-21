```text
███████╗ ██████╗ ██████╗  ██████╗ ███████╗
██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
█████╗  ██║   ██║██████╔╝██║  ███╗█████╗
██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
    >> gdbforge: Extreme Tooling Suite <<
```

# gdbforge

**gdbforge** is a Vim-inspired multi-pane terminal front-end for GDB. It keeps GDB’s power in the TTY and adds a workspace for source, GDB console, IO (program stdin/stdout), threads, call stack, and breakpoints — with modes, a `:` command line, and mouse/clipboard support.

## Typical use cases

- Debugging **Linux applications** with GDB without leaving the terminal
- **Embedded Linux** and board bring-up (source + GDB + process panes together)
- **MCU / cross** workflows where layout and keyboard speed matter
- Teams who want a **Vim-like** flow: normal / insert / command modes, panes, buffers

## Demo

<!-- TODO: add screencast GIF or short MP4 -->
![gdbforge demo](docs/media/gdbforge-demo.gif)

## Problems it solves

- Switching constantly between a raw GDB TTY and a separate editor/viewer
- Weak or fixed pane layouts for source, console, and process state
- Limited mouse and clipboard in classic terminal debugger UIs
- Hard-to-discover keys and no in-app manual
- Accidental resume of a running inferior when changing frames/threads

## What you get

- Named layouts (`:layout wide`, `panels`, `default`, `classic`) and splits (`:vs` / `:split`)
- Code (or startup logo), GDB console, IO, Threads, Call Stack, Breakpoints
- In-app manual: `:help` or `:b help`
- Space to toggle breakpoints on the Code cursor or Call Stack frame
- Status colors for insert (green) vs normal (blue)
- Cmdline and console paste: Ctrl+V, middle-click; selection copy/cut
- Safer while-running behavior: auto-continue after breakpoint **insert** only (not after frame/thread switches)

Deeper architecture notes live under [docs/](docs/) — start with [MVC](docs/ARCHITECTURE.md#mvc-current).

## Install and run (PC)

### Requirements

- Linux PC (or similar) with a terminal
- [Go](https://go.dev/dl/) (module requires a recent Go toolchain)
- `gdb` and `gcc` on `PATH` (`sudo apt install gdb gcc` on Debian/Ubuntu)

### Download

```bash
git clone https://github.com/yairgd/gdbforge.git
cd gdbforge
```

### Build gdbforge

```bash
go build -o bin/gdbforge ./cmd/gdbforge
# or: make build
```

### Hello world example

Build a tiny C program with debug info and **no** optimization so stepping and breakpoints behave as expected:

```bash
cat > hello.c <<'EOF'
#include <stdio.h>

int main(void) {
    printf("hello, gdbforge\n");
    return 0;
}
EOF

gcc -O0 -g -o hello hello.c
```

Run it under gdbforge:

```bash
./bin/gdbforge ./hello
# or without installing the binary first:
go run ./cmd/gdbforge ./hello
```

Inside the app: press `i` to type in the GDB console, or use `:` for commands. Open the manual with `:help`. Quit with `:quit` or Ctrl-D.

Optional: pick another GDB binary with `-d`:

```bash
./bin/gdbforge -d /usr/bin/gdb ./hello
```

---

## User guide

Same content as the in-app manual (`:help` / `:b help`). Keep this section aligned with `buildHelpLines()` in `internal/gdbforge/widgets/help_widget.go`.

### Overview

gdbforge (gdbforge: Extreme Tooling Suite) is a Vim-inspired terminal debugger built on GDB. The screen is a multi-pane workspace: Code (or logo), GDB console, IO, Threads, Call Stack, Breakpoints, plus a global `:` command line at the bottom.

**Status line colors:**

- Green — insert mode (pane actively typing / focused for edit)
- Blue — normal mode (pane remembered / not insert-active)

### Modes

**Normal**

Default navigation mode. Global keys (`n`/`s`, Space, window focus) and focused-pane keys apply. Esc leaves insert into normal.

**Insert**

Type into the focused console (usually GDB). Enter with `i` (focuses GDB) or by clicking a pane. Esc returns to normal. When GDB is focused, printable keys (including Space) go to GDB.

**Command**

Enter with `:` or by clicking the bottom cmdline. Type a command, Tab for completion, Enter to run, Esc to cancel.

**Completion**

After Tab with multiple matches, the wildmenu opens above `:`. Left/Right/Tab cycle; type letters to narrow (CompletionMenu); Enter accepts; Esc cancels. The completion bar is a replaceable view over that menu.

### Global keys (normal mode)

| Key | Action |
|-----|--------|
| `:` | enter command mode |
| `i` | focus GDB leaf and enter insert |
| `Esc` | leave insert; focus last non-Code/non-GDB pane if one was active, else the Code/logo leaf (`:set noesctocode` keeps focus on current pane) |
| `n` | GDB next (also in insert when Code is focused) |
| `s` | GDB step (also in insert when Code is focused) |
| Up / Down | move Code cursor line (global, with fallthrough) |
| Space | toggle breakpoint: Call Stack selection, or Code cursor line (never steals Space from GDB) |
| `e` | enable/disable breakpoint at Code cursor |
| Ctrl-W h/j/k/l | focus left / down / up / right |
| Ctrl-W arrows | same as hjkl |
| Ctrl-O | jump back after `:b` / `:edit` / `:!` |
| Ctrl-D | quit (TermApp) |

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
- `breakmain` / `nobreakmain` — insert `break main` on GDB start (default on)
- `gdblistenprint` / `nogdblistenprint` — paint App/MCP replies in GDB console (default on)
- `gdbtargetprint` / `nogdbtargetprint` — also paint program stdout in GDB console like a classic terminal (default off); IO pane always uses the inferior PTY
- `markcolor <name>` — focused list selection (default blue)
- `markdimcolor <name>` — unfocused selection (default gray)
- `breakcolor <name>` — enabled BP bg (default red)
- `breakdisabledcolor <name>` — disabled BP bg (default yellow)

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
- Up/Down or j/k — bold cursor line
- Space — insert/remove breakpoint at cursor
- `e` — enable/disable BP (yellow gutter when disabled)
- Missing/.so src — centered "not available" + path
- Status line — full file path when focused

**GDB console**

- Insert mode to type CLI; Enter submits; Ctrl-C interrupt; Ctrl-D sends `q`; Ctrl-L clear; Ctrl-V / middle-click paste
- `frame` / `f` / `up` / `down` sync Code + Call Stack after `(gdb)` prompt
- Mouse drag selects scrollback; Ctrl-C copies selection

**Breakpoints (`:b breakpoint`)**

- j/k or Up/Down / Enter / click — select and show source
- `e` — toggle enable (disabled rows stay in list, removed from GDB)
- `d` — delete from list and GDB
- Row colors from `breakcolor` / `breakdisabledcolor`

**Threads (`:b threads`)**

- j/k Up/Down Enter click wheel — select thread, send `thread <id>`, refresh stack, show current frame source

**Call Stack (`:b callstack`)**

- j/k Up/Down Enter click wheel — select frame, send `frame N`, show source (or not-available placeholder)
- Space — toggle breakpoint at selected frame location

**IO console (`:b io`, alias `:b output`)**

- Program stdin/stdout on a **dedicated PTY** (`ptyx.TTY` + GDB `-inferior-tty-set`); GDB uses a separate MI PTY
- Type here while the inferior is running; Enter sends to the program
- PgUp/PgDn scroll; Ctrl-L clear; Ctrl-C / Ctrl-D → program interrupt / EOF; ANSI colors
- GDB console input never reaches the program — see [docs/DEBUGGER_INTEGRATION.md](docs/DEBUGGER_INTEGRATION.md#inferior-io-dual-pty)

**About (`:b about`)**

- Version, build info, and license

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

### See also (in-app)

- `:b about` — version and license
- `:help` — this manual

---

## License

MIT License

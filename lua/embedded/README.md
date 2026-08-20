---
description: Embedded Linux and user-space app debug with gdbforge — remotegdb, internal IO pane, external terminal stdio, gdbserver workflows.
meta:
  - name: keywords
    content: embedded Linux debugger, gdbserver remote debug, inferior tty, external terminal debug, GDB IO pane, ARM Linux GDB, gdbforge, remote target debug
---

# Embedded Linux & user-space app debugging

Lua workflows for **user-space programs** — on an **embedded Linux board** or **locally on the host**. Includes **where program stdin/stdout goes**: the built-in **IO pane** (`:b io`), a **real terminal window**, or **gdbserver** (local or remote).

Full guide: **[EMBEDDED_LINUX_DEBUG.md](../../docs/EMBEDDED_LINUX_DEBUG.md)**

## Program I/O — three options

| Where I/O goes | When to use | How |
|----------------|-------------|-----|
| **Internal IO pane** | Simple printf, line input, low traffic | Default — `:b io` (alias `:b output`). Type while the inferior runs. |
| **External terminal** | Full-screen TUI, curses, high-rate stdout | `:lua external_tty` or `:lua terminal_debug` or `:set inferior-tty` |
| **gdbserver terminal** | Debug via RSP; inferior lives in gdbserver's tty | `:lua gdbserver_tui` (local) or `:lua remotegdb` (board) |

When stdio is **external**, `:b io` shows a note only — interact in the other window. Restore internal IO: `:set inferior-tty internal`.

```bash
export GDBFORGE_TERMINAL=mate-terminal   # for external terminal scripts
```

## Quick start — board (`:lua remotegdb`)

```bash
mkdir -p .gdbforge/lua
cp -r lua/embedded/remotegdb .gdbforge/lua/
export GDBFORGE_REMOTE_HOST=192.168.20.50
export GDBFORGE_TERMINAL=mate-terminal
./bin/gdbforge ./hello
:lua remotegdb
```

## Quick start — local app, external terminal

```bash
cp -r lua/embedded/terminal_debug .gdbforge/lua/
export GDBFORGE_TERMINAL=mate-terminal
./bin/gdbforge ./my_tui
:lua terminal_debug ./my_tui run
```

## Script catalog

| `:lua` | I/O | Purpose |
|--------|-----|---------|
| `remotegdb` | External (gdbserver on board via SSH) | scp + ssh gdbserver + `target remote` |
| `remotegdb_log` | — | Tail board gdbserver log in a terminal |
| `gdbserver_tui` | External (local gdbserver window) | Local `gdbserver` + `target remote` |
| `terminal_debug` | External | Open tty + optional `file` / `break` / `run` |
| `external_tty` | External | Open tty only; you `file` / `run` yourself |
| _(none)_ | **Internal** `:b io` | Default — no script needed |

## Install

Copy the folders you need (flat into `.gdbforge/lua/`):

```bash
cp -r lua/embedded/remotegdb .gdbforge/lua/        # board
cp -r lua/embedded/terminal_debug .gdbforge/lua/   # external tty + load
cp -r lua/embedded/external_tty .gdbforge/lua/     # external tty only
cp -r lua/embedded/gdbserver_tui .gdbforge/lua/    # local gdbserver
```

## Environment variables

| Variable | Used by | Meaning |
|----------|---------|---------|
| `GDBFORGE_TERMINAL` | external tty, gdbserver, remotegdb | Terminal emulator (`mate-terminal`, `kitty`, …) |
| `GDBFORGE_INFERIOR_TTY` | startup override | Force pts at gdbforge launch |
| `GDBFORGE_REMOTE_*` | remotegdb | Board host, user, port, app path — see [EMBEDDED_LINUX_DEBUG.md](../../docs/EMBEDDED_LINUX_DEBUG.md) |

## Common questions

### How to debug an embedded Linux app with GDB?

`:lua remotegdb` — see [EMBEDDED_LINUX_DEBUG.md](../../docs/EMBEDDED_LINUX_DEBUG.md).

### Program output in gdbforge vs another window?

- **Inside gdbforge:** default `:b io` pane (internal PTY).
- **Separate window:** `:lua external_tty` then `run`, or `:set inferior-tty`.
- **After remotegdb:** program UI runs on the **board**; gdbserver SSH session may show inferior output in the host terminal.

### TUI / curses app won't work in `:b io`?

Use **external terminal** — `:lua terminal_debug` or `:set inferior-tty`. Full VT100/curses needs a real terminal emulator.

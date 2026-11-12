---
title: Debugging Python in the Terminal — debugpy Debugger with Vim-style Panes
description: Debug Python scripts, modules and pytest tests in the terminal with gdbforge and debugpy — breakpoints, stepping, call stack, threads, and Python evaluated in the paused frame.
---

# Debugging Python in the terminal

`gdbforge -g py` is a terminal debugger for Python with the same multi-pane, keyboard-driven layout
gdbforge uses for C and Go: source with a breakpoint gutter and current-line arrow, call stack, threads,
breakpoints, the program's output in its own pane, and a `(py)` console where you can read and change
variables in the paused frame. It drives [debugpy](https://github.com/microsoft/debugpy) over the Debug
Adapter Protocol, so it works over SSH and in a plain terminal, with no IDE.

## Requirements

```bash
python3 -m pip install debugpy      # in the interpreter you will debug with
```

gdbforge checks `import debugpy` at start and prints this hint if it fails. To use a virtualenv, pass its
interpreter with `-d`: `gdbforge -g py -d ~/venv/bin/python script.py`.

## Quick start

```bash
gdbforge -g py script.py [args…]                    # a script
gdbforge -g py -- -m mypkg.cli [args…]              # a module
gdbforge -g py -- -m pytest tests/test_x.py::test_foo   # a single pytest test
```

A script starts stopped at its first statement. Set a breakpoint with `Space` in the Code pane or
`b file.py:42` in the console, then `c` to continue. Modules (`-m`) have no entry stop: set a breakpoint
first, then `c`.

## Keys and the `(py)` console

Stepping and breakpoint keys are the same as for GDB and Delve (see the [User guide](USER_GUIDE.md)):
`n` next, `s` step in, `c` continue, `f` finish, `Space` toggle a breakpoint, Ctrl-C to pause.

In the console, debugger words win and anything else is Python evaluated in the selected frame
(prefix with `!` to force Python):

| Command | Action |
|---------|--------|
| `b` / `break [file:line \| line \| func]` | Set a breakpoint (no argument: list) |
| `tbreak` | Breakpoint removed at its first hit |
| `clear` · `delete N` · `disable N` · `enable N` · `condition N [expr]` | Manage breakpoints |
| `c` · `n` · `s` · `finish` · `pause` | Continue, step over, step in, step out, pause |
| `up` · `down` · `frame N` · `bt` | Move in the call stack / list it |
| `threads` · `thread ID` · `breakpoints` · `files` | Inspect |
| `restart` | Kill, respawn, re-attach and resend breakpoints |
| `quit` | Exit |

```text
(py) b script.py:12
(py) c
> compute() script.py:12
(py) dx
3.5
(py) dx = 99
(py) n
```

Program output goes to `:b io`. Pane contents refresh at each stop, not while the program runs.

## Limits

- No assembly view, registers, memory reads, watchpoints or address breakpoints; no remote attach or
  multi-process debugging.
- `--call pkg.mod:func` is parsed but not wired yet.
- Large values (for example DataFrames) are truncated in the console.
- The REST/MCP run-control tools are not connected to the Python backend yet.
- The program's terminal is chosen at spawn; `:set inferior-tty` is not supported.

## How it works

gdbforge starts `python -m debugpy --listen 127.0.0.1:PORT --wait-for-client`, attaches over DAP and keeps
the breakpoint table itself. Technical details: [Debugger integration](DEBUGGER_INTEGRATION.md#python-backend-peer-of-gdb-and-delve)
and [PTY architecture](PTY_ARCHITECTURE.md#mode-e--python-debugpy).

For Go, see [the FAQ](FAQ.md#can-i-debug-go-programs); for C and embedded targets, the other
[debugging guides](EMBEDDED_LINUX_DEBUG.md).

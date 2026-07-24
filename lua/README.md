# gdbforge Lua scripts

**`gdbforge.*` API reference:** [docs/LUA_API.md](../docs/LUA_API.md). In-app: `:help` (Lua section).

`lua/` is the **bring-up and workflow library** for gdbforge: a living directory of
installable Lua plugins (plus sidecar data). It is meant to **grow over time** as we
add boards, CPUs, and debug setups — not a one-off examples folder.

## One directory per setup

Every board, every CPU, and every distinct workflow gets **its own subdirectory**.
That directory owns everything needed to attach and debug that target:

| Kind of content | Examples |
|-----------------|----------|
| Lua entrypoint | `r5_debug.lua` → `:lua r5_debug` |
| GDB target description | `r5_target.xml` |
| Probe / server flags | J-Link device name, speed, GDB port |
| Notes / env overrides | `GDBFORGE_JLINK`, `GDBFORGE_TDESC`, … |

Do **not** dump unrelated targets into one mega-script. Copy only the directory you
need into a project’s `.gdbforge/lua/`.

Suggested naming as the tree grows:

```text
lua/
  terminal_debug/          # host TUI / external tty (any program)
  gdbserver_tui/           # pattern: gdbserver in other terminal
  dlv_port/                 # pattern: Go + headless dlv
  games/                   # demos (snake, tetris)
  r5_debug/                # Cortex-R5 + J-Link (XCZU3CG_R5_0 today)
  # future — same idea, new dirs:
  # a53_debug/             # Cortex-A53 bring-up
  # stm32f4_debug/         # board-specific OpenOCD/J-Link
  # nrf52_debug/           # …
```

When you add a new CPU or board: create `lua/<name>/`, put the Lua + XML (or
OpenOCD cfg, SVD snippets, etc.) **beside** each other, document env vars in the
Lua header, and list the directory in the table below.

`gdbforge.lua_dir()` returns that script’s directory so sidecars resolve correctly
even when nested under `.gdbforge/lua/<name>/`.

## Install into a project

From the project you will debug (cwd for gdbforge):

```bash
mkdir -p .gdbforge/lua
cp -r /path/to/gdbforge/lua/terminal_debug .gdbforge/lua/
cp -r /path/to/gdbforge/lua/r5_debug .gdbforge/lua/
# optional demos:
cp -r /path/to/gdbforge/lua/games .gdbforge/lua/
```

Or copy everything:

```bash
mkdir -p .gdbforge/lua
cp -r /path/to/gdbforge/lua/* .gdbforge/lua/
```

Then inside gdbforge: `:lua <name>` (Tab completes). Command name = Lua file basename.

## Catalog (current)

| Directory | `:lua` command | Notes |
|-----------|----------------|-------|
| `terminal_debug/` | `terminal_debug` | Open external tty for GDB/Go TUI stdio |
| `external_tty/` | `external_tty` | Same idea, no `file`/`break` |
| `gdbserver_tui/` | `gdbserver_tui` | gdbserver in other terminal + `target remote` |
| `dlv_port/` | `dlv_port` | Headless dlv on port + `dlv_connect` (program from session) |
| `dlv_ext_port/` | `dlv_ext_port` | Alias of `dlv_port` |
| `r5_debug/` | `r5_debug` | J-Link + `r5_target.xml` (Cortex-R5) |
| `games/snake/` | `snake` | Opens builtin Snake pane (`:b snake`) |
| `games/tetris/` | `tetris` | Opens builtin Tetris pane (`:b tetris`) |

Layout example after install:

```text
.gdbforge/lua/
  terminal_debug/terminal_debug.lua
  r5_debug/r5_debug.lua
  r5_debug/r5_target.xml
  games/snake/snake.lua
  games/tetris/tetris.lua
```

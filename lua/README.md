# gdbforge Lua scripts

**API:** [docs/LUA_API.md](../docs/LUA_API.md) · **In-app:** `:help` (Lua section) · **User guide:** [docs/USER_GUIDE.md](../docs/USER_GUIDE.md)

Installable workflows under `lua/`. Copy what you need into a project’s `.gdbforge/lua/`, then `:lua <name>` (Tab completes). Command name = Lua file basename.

---

## Install

```bash
mkdir -p .gdbforge/lua
cp -r /path/to/gdbforge/lua/remotegdb .gdbforge/lua/
cp -r /path/to/gdbforge/lua/dlv_ext_port .gdbforge/lua/
# or everything:
# cp -r /path/to/gdbforge/lua/* .gdbforge/lua/
```

---

## Shared environment variables

These apply to **any** script that opens a real terminal (`spawn_terminal`, `open_external_tty`, `spawn_dlv_headless`):

| Variable | Meaning | Example |
|----------|---------|---------|
| **`GDBFORGE_TERMINAL`** | Terminal emulator binary (or known name) | `export GDBFORGE_TERMINAL=mate-terminal` |
| `GDBFORGE_INFERIOR_TTY` | Force inferior pts at gdbforge startup (`/dev/pts/N`) | mostly Delve spawn-time `--tty` |

Supported / auto-detected names include: `kitty`, `mate-terminal`, `gnome-terminal`, `xterm`, `konsole`, `alacritty`. If unset, gdbforge picks the first found on `PATH`.

```bash
export GDBFORGE_TERMINAL=mate-terminal
~/gdbforge/bin/gdbforge -g dlv ./hello-go
# then :lua dlv_ext_port 1234   → opens mate-terminal
```

Per-script env vars are listed in each section below.

---

## Catalog

| Directory | `:lua` | Backend | Purpose |
|-----------|--------|---------|---------|
| [`remotegdb/`](remotegdb/) | `remotegdb` | **GDB** | Embedded Linux: scp (if changed) + ssh gdbserver + `target remote` |
| [`dlv_ext_port/`](dlv_ext_port/) | `dlv_ext_port` | Delve | Go TUI: headless dlv in another window + connect |
| [`dlv_port/`](dlv_port/) | `dlv_port` | Delve | Alias of `dlv_ext_port` |
| [`terminal_debug/`](terminal_debug/) | `terminal_debug` | GDB | External tty + optional `file` / `break` / `run` |
| [`external_tty/`](external_tty/) | `external_tty` | GDB | External tty only |
| [`gdbserver_tui/`](gdbserver_tui/) | `gdbserver_tui` | GDB | Local `gdbserver` in another window + `target remote` |
| [`r5_debug/`](r5_debug/) | `r5_debug` | GDB | Cortex-R5 + J-Link bring-up |
| [`games/snake/`](games/snake/) | `snake` | — | Demo pane |
| [`games/tetris/`](games/tetris/) | `tetris` | — | Demo pane |

---

## `remotegdb` — embedded Linux board (scp + gdbserver)

Deploy a host binary to the board, start `gdbserver` over SSH in an external terminal, attach GDB.

### Setup

```bash
mkdir -p .gdbforge/lua
cp -r lua/remotegdb .gdbforge/lua/

# SSH key login recommended (BatchMode). Example board:
#   root@192.168.20.50  with gdbserver on PATH
export GDBFORGE_TERMINAL=mate-terminal
export GDBFORGE_REMOTE_HOST=192.168.20.50
export GDBFORGE_REMOTE_USER=root
export GDBFORGE_REMOTE_PORT=1234
export GDBFORGE_REMOTE_APP=./hello   # optional if you pass argv / start with the binary

./bin/gdbforge ./hello               # GDB backend (default)
# then:
:lua remotegdb
:lua remotegdb ./hello
:lua remotegdb ./hello 192.168.20.50 1234
```

### What it does

1. Resolves **local app**: `:lua` arg → `GDBFORGE_REMOTE_APP` → `gdbforge.program()` → `DEFAULT_APP` in the script  
2. MD5 local vs remote (`ssh … md5sum`); **scp only if missing or different**  
3. `chmod +x` on the remote path (default `/tmp/<basename>`)  
4. `spawn_terminal`: `ssh -t user@host 'gdbserver :PORT /tmp/app'`  
5. `wait_port("host:PORT")` then `file <app>` + `target remote host:PORT`

Edit placeholders at the top of [`remotegdb/remotegdb.lua`](remotegdb/remotegdb.lua) (`DEFAULT_HOST`, `DEFAULT_APP`, …) if you prefer not to use env vars.

### Env

| Variable | Default | Meaning |
|----------|---------|---------|
| `GDBFORGE_REMOTE_APP` | _(empty)_ | Local binary path |
| `GDBFORGE_REMOTE_HOST` | `192.168.20.50` | Board IP / hostname |
| `GDBFORGE_REMOTE_USER` | `root` | SSH user |
| `GDBFORGE_REMOTE_PORT` | `1234` | gdbserver listen port on the board |
| `GDBFORGE_REMOTE_DIR` | `/tmp` | Remote directory for the binary |
| `GDBFORGE_TERMINAL` | auto | Terminal for the ssh/gdbserver window |

### Args

| `:lua remotegdb …` | Meaning |
|--------------------|---------|
| _(none)_ | All from env / defaults / session program |
| `APP` | Local path |
| `APP HOST` | Path + board |
| `APP HOST PORT` | Path + board + gdbserver port |

Requires host tools: `ssh`, `scp`, `md5sum` (or `md5`); board: `gdbserver`.

---

## `dlv_ext_port` / `dlv_port` — Go + Delve external terminal

Preferred Go / TUI flow (Delve `--tty` is spawn-only).

```bash
export GDBFORGE_TERMINAL=mate-terminal
~/gdbforge/bin/gdbforge -g dlv ./hello-go
# install: cp -r lua/dlv_ext_port .gdbforge/lua/
:lua dlv_ext_port 1234
:lua dlv_ext_port 1234 --flag arg   # extra args after the binary
:lua dlv_port                       # alias; default port 2345
```

1. Opens headless Delve in another terminal on `127.0.0.1:PORT`  
2. Waits for the port  
3. `dlv connect` — inferior I/O stays in that window  

| Variable | Meaning |
|----------|---------|
| `GDBFORGE_TERMINAL` | Terminal emulator for headless dlv |

Program path = `gdbforge.program()` (must start with `-g dlv` and a binary).

---

## `terminal_debug` — GDB external tty + optional load

```bash
export GDBFORGE_TERMINAL=mate-terminal
./bin/gdbforge ./my_tui
:lua terminal_debug                 # tty only
:lua terminal_debug ./my_tui        # file + break main
:lua terminal_debug ./my_tui run    # also run
```

| Variable | Meaning |
|----------|---------|
| `GDBFORGE_TERMINAL` | Emulator for the hold-open tty window |
| `GDBFORGE_INFERIOR_TTY` | Optional pts override at process start |

For **Delve**, prefer `dlv_ext_port` instead.

---

## `external_tty` — GDB tty only

```text
:lua external_tty
```

Opens an external terminal and `set_inferior_tty`. No `file`/`break`. Same `GDBFORGE_TERMINAL` / `GDBFORGE_INFERIOR_TTY` as above.

---

## `gdbserver_tui` — local gdbserver (host TUI)

```text
:lua gdbserver_tui                 # ./hello , port 2345
:lua gdbserver_tui ./my_tui 2345
```

Spawns `gdbserver :PORT prog` in an external terminal, waits on **localhost**, `target remote localhost:PORT`.

| Variable | Meaning |
|----------|---------|
| `GDBFORGE_TERMINAL` | Emulator for the gdbserver window |

For a **remote board**, use `remotegdb` instead.

---

## `r5_debug` — Cortex-R5 + J-Link

```bash
cp -r lua/r5_debug .gdbforge/lua/    # includes r5_target.xml
:lua r5_debug
```

Background `JLinkGDBServer` (Code pane stays), `wait_port`, then architecture / tdesc / `target remote` / `load` / `break main`.

| Variable | Default role |
|----------|----------------|
| `GDBFORGE_JLINK` | Path to `JLinkGDBServer` |
| `GDBFORGE_JLINK_DEVICE` | e.g. `XCZU3CG_R5_0` |
| `GDBFORGE_JLINK_PORT` | GDB port (default `2334`) |
| `GDBFORGE_TDESC` | Target description XML (default: script dir `r5_target.xml`) |

Optional: `:b exec` for J-Link logs. Does **not** use `GDBFORGE_TERMINAL` (background spawn).

---

## `snake` / `tetris` — demos

```text
:lua snake
:lua tetris
```

Opens builtin game panes. No debugger env vars.

---

## Recommended quick recipes

**Go TUI (Delve)**

```bash
export GDBFORGE_TERMINAL=mate-terminal
~/gdbforge/bin/gdbforge -g dlv ./hello-go
:lua dlv_ext_port 1234
```

**Embedded Linux board (GDB)**

```bash
export GDBFORGE_TERMINAL=mate-terminal
export GDBFORGE_REMOTE_HOST=192.168.20.50
./bin/gdbforge ./hello
:lua remotegdb
```

**Host GDB TUI**

```bash
export GDBFORGE_TERMINAL=mate-terminal
./bin/gdbforge ./my_tui
:lua terminal_debug ./my_tui
```

---

## Layout after install

```text
.gdbforge/lua/
  remotegdb/remotegdb.lua
  dlv_ext_port/dlv_ext_port.lua
  terminal_debug/terminal_debug.lua
  r5_debug/r5_debug.lua
  r5_debug/r5_target.xml
  games/snake/snake.lua
  games/tetris/tetris.lua
```

---

## Adding a new workflow

1. Create `lua/<name>/<name>.lua` with `function main(...)`.
2. Document usage + env vars in the file header and this README.
3. One directory per board/CPU/workflow.

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
| [`cortex_r5/`](cortex_r5/) | `r5_baremetal_jlink` | GDB | Cortex-R5 + J-Link bare-metal bring-up |
| [`cortex_r5/`](cortex_r5/) | `r5_baremetal_openocd_digilent` | GDB | Cortex-R5 + Digilent HS2 OpenOCD bare-metal |
| [`cortex_r5/`](cortex_r5/) | `r5_openamp_jlink` | GDB | Cortex-R5 OpenAMP + J-Link attach |
| [`cortex_r5/`](cortex_r5/) | `r5_openamp_openocd_digilent` | GDB | Cortex-R5 OpenAMP + Digilent HS2 OpenOCD attach |
| [`gvim/`](gvim/) | `gvim` | — | Open CodeWidget file in gVim (`--servername`, new tab) |
| [`vim/`](vim/) | `vim` | — | Open CodeWidget file in terminal vim (over gdbforge; resumes on quit) |
| [`vscode/`](vscode/) | `vscode` | — | Open CodeWidget file in VS Code / VSCodium (+ project workspace) |
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
export GDBFORGE_REMOTE_APP_ARGS='-p /dev/ff/ -z'  # optional inferior argv for gdbserver

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
4. Starts `gdbserver :PORT /tmp/app [GDBFORGE_REMOTE_APP_ARGS…]` on the board (stdout → log; optional `ssh tail`)  
5. `wait_port("host:PORT")` then `file <app>` + `target remote host:PORT`

Edit placeholders at the top of [`remotegdb/remotegdb.lua`](remotegdb/remotegdb.lua) (`DEFAULT_HOST`, `DEFAULT_APP`, …) if you prefer not to use env vars.

### Env

| Variable | Default | Meaning |
|----------|---------|---------|
| `GDBFORGE_REMOTE_APP` | _(empty)_ | Local binary path |
| `GDBFORGE_REMOTE_APP_ARGS` | _(empty)_ | Inferior argv after the binary (`gdbserver :PORT prog [args…]`), e.g. `-p /dev/ff/ -z` |
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

## `cortex_r5` — Cortex-R5 (J-Link / Digilent OpenOCD)

```bash
cp -r lua/cortex_r5 .gdbforge/lua/    # includes r5_target.xml + OpenOCD cfg
:lua r5_baremetal_jlink
:lua r5_baremetal_openocd_digilent
:lua r5_openamp_jlink ./firmware
:lua r5_openamp_openocd_digilent ./firmware
```

Background probe GDB server (Code pane stays), `wait_port`, then architecture / tdesc / `target remote` / `load` (bare-metal) / `break main`.

| Variable | Default role |
|----------|----------------|
| `GDBFORGE_R5_CORE` | RPU core `0`/`1`/`R0`/`R1` (default **R0**) — J-Link `…_R5_N`, OpenOCD target, `remoteprocN` |
| `GDBFORGE_JLINK_CHIP` | J-Link chip prefix (default `XCZU3CG`) → `CHIP_R5_N` when `JLINK_DEVICE` unset |
| `GDBFORGE_JLINK` | Path to `JLinkGDBServer` |
| `GDBFORGE_JLINK_DEVICE` | Full override e.g. `XCZU3CG_R5_0` (else `CHIP_R5_N`; trailing `_R5_N` from `R5_CORE`) |
| `GDBFORGE_JLINK_PORT` | J-Link GDB port (default `2334`) |
| `GDBFORGE_OPENOCD` | Path to `openocd` (default: `openocd` on `PATH`) |
| `GDBFORGE_OPENOCD_CFG` | Digilent HS2 + R5 cfg (default: script dir `r5_openocd_digilent.cfg`) |
| `GDBFORGE_OPENOCD_PORT` | OpenOCD GDB port (default `3333`) |
| `GDBFORGE_TDESC` | Target description XML (default: script dir `r5_target.xml`) |

Optional: `:b exec` for probe logs. Does **not** use `GDBFORGE_TERMINAL` (background spawn).

---

## `gvim` — open Code source in gVim

Reuse one gVim instance (`--servername`); each `:lua gvim` opens the CodeWidget **blue pointer** line (browse cursor — same as j/k; on PC after a stop) in a **new tab**.

```text
:lua gvim                 # blue pointer line
:lua gvim hello.c 42      # optional path + line
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `GDBFORGE_GVIM_SERVER` | `GDBFORGE` | `--servername` id (same app across calls) |
| `GDBFORGE_GVIM` | `gvim` | Binary on `PATH` |

Requires host `gvim` with `+clientserver` (typical GUI builds).

---

## `vim` — open Code source in terminal vim

Suspends the gdbforge TUI and runs terminal `vim` on the same tty. When vim exits (`:q`), gdbforge resumes and redraws.

```text
:lua vim                  # blue pointer line
:lua vim hello.c 42       # optional path + line
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `GDBFORGE_VIM` | `vim` | Binary on `PATH` (`nvim`, …) |

Uses `gdbforge.foreground(VIM, "+LINE", FILE)`. Prefer `:lua gvim` / `:lua vscode` for a separate GUI editor.

---

## `vscode` — open Code source in VS Code / VSCodium

Reuse the editor window; each `:lua vscode` opens/focuses the CodeWidget **blue pointer** line (browse cursor — on PC after a stop) and, when possible, opens the **project folder as the workspace**.

```text
:lua vscode                 # blue pointer line (+ workspace if detected)
:lua vscode hello.c 42      # optional path + line
```

Auto-detects the first of `code` / `codium` on `PATH`.

Workspace root (first match walking up from the file):

- `.git/`
- `.gdbforge/`
- `go.mod`
- `CMakeLists.txt`

If none is found, opens the file only.

| Variable | Default | Meaning |
|----------|---------|---------|
| `GDBFORGE_VSCODE` | _(auto)_ | Force binary (`code` or `codium`) |
| `GDBFORGE_VSCODE_WORKSPACE` | _(auto)_ | Force folder or `.code-workspace` path |

Uses `BIN -r [WORKSPACE] -g FILE:LINE` (reuse window + goto line).

---

## `snake` / `tetris` — demos

```text
:lua snake
:lua tetris
# then: :b snake / :b tetris to refocus (not listed until created)

# second independent game (separate VM / state):
:lua snake snake1
:b snake1
```

Create-or-focus ModeLua game panes via `:lua` (no boot-time game widgets). Same buffer name shares one game; a different name (`snake1`) is a new instance. No debugger env vars.

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

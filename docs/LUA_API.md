# Lua API (`gdbforge.*`)

User-facing reference for scripting gdbforge.

**Install split:** Framework helpers (`print`, `register`, `spawn`, `pane.*`, …) ship with `luahost`. Debugger bindings (`gdb`, `dlv_connect`, `spawn_dlv_headless`, `set_inferior_tty`, `program`) are installed by `cmd/gdbforge` via `wireUserLuaAPI` — they are present in the debugger app, not in a bare `luahost.New` runtime.
 In-app summary: **`:help`** (Lua section). Architecture/status: [PLUGINS.md](PLUGINS.md). Installable workflows: [../lua/README.md](../lua/README.md).

Scripts live under `./.gdbforge/lua/**/*.lua` (nested dirs OK). Each file gets its own Lua VM. Basename without `.lua` is the `:lua` command name (e.g. `r5_debug/r5_debug.lua` → `:lua r5_debug`).

```bash
mkdir -p .gdbforge/lua
cp -r /path/to/gdbforge/lua/terminal_debug .gdbforge/lua/
# inside gdbforge:
#   :lua terminal_debug
```

---

## Lifecycle

| Hook | When |
|------|------|
| Top-level script body | Runs once when the script is loaded / `:lua name` first needs it |
| `gdbforge.register("name", fn)` | Exposes `fn` as `:lua name [args…]` |
| `main(...)` (optional convention) | Many catalog scripts define `main` and register it |
| ModeLua pane (`:b lua` / games) | `on_key`, draw via `pane.*`, optional tick — see games under `lua/games/` |

`:lua <func> [args]` calls a registered function; string args are passed as Lua strings.

---

## Core helpers

### `gdbforge.print(...)`

Append a line to the Lua / messages pane (coerces args like `print`).

### `gdbforge.clear()`

Clear the Lua pane buffer.

### `gdbforge.register(name, fn)`

Register `fn` so `:lua name` invokes it. `name` must be a non-empty string; `fn` a Lua function.

### `gdbforge.sleep(seconds)`

Block the Lua VM for `seconds` (number, may be fractional). Used while waiting for ports/probes.

### `gdbforge.lua_dir()`

Absolute directory of the **current** script file (for sidecars: XML, configs next to the `.lua`). Safe to call from registered functions without deadlocking the runtime.

### `gdbforge.program()`

Debuggee path from the current gdbforge session (may be `""` if none). Prefer this over hard-coding binaries in scripts that attach to the session program.

### `gdbforge.wait_port(host_port [, timeout_sec])`

Block until TCP accepts connections, or until timeout (default ~10s, max 120s). Returns `true`/`false`.

| Argument | Meaning |
|----------|---------|
| `"1234"` | Wait on `127.0.0.1:1234` |
| `"192.168.20.50:1234"` | Wait on that host:port (embedded board / remote gdbserver) |

Typical after `spawn` / `spawn_terminal` / `spawn_dlv_headless` / `:lua remotegdb`.

---

## Debugger console

### `gdbforge.gdb(cmd)`

Send one CLI line to the active debugger console (GDB or Delve), same path as typing in the GDB pane.

```lua
gdbforge.gdb("break main")
gdbforge.gdb("continue")
```

Under Delve (`-g dlv`), send Delve CLI (`break main.main`, `continue`, …), not GDB MI.

### `gdbforge.open_buffer(name)`

Focus/open a builtin or buffer by name without blindly stealing the wrong leaf when possible: `"code"`, `"gdb"`, `"lua"`, `"io"`, `"callstack"`, `"snake"`, …

---

## Process helpers

### `gdbforge.spawn(argv...)`

Start a **background** PTY process (argv as separate string args or a table). Does **not** steal focus from Code — use for J-Link GDB servers, helpers, etc. Output may appear under `:b exec` if wired.

```lua
gdbforge.spawn("JLinkGDBServer", "-device", "XCZU3CG_R5_0", "-if", "JTAG", "-port", "2331")
```

### `gdbforge.run(argv...)`

Interactive shell / `:!` path — replaces the focused pane with an Exec session. Prefer `spawn` when you must keep Code focused.

### `gdbforge.spawn_terminal(argv...)`

Open a **real terminal emulator** running `argv`. Emulator selection:

```bash
export GDBFORGE_TERMINAL=mate-terminal   # or kitty|xterm|gnome-terminal|…
```

If unset, the first of kitty / mate-terminal / gnome-terminal / xterm / … on `PATH` is used.

```lua
gdbforge.spawn_terminal("gdbserver", ":2345", "./my_tui")
gdbforge.spawn_terminal("ssh", "-t", "root@192.168.20.50", "gdbserver :1234 /tmp/hello")
```

---

## Inferior stdio / external TTY

`:b io` is a **line console**, not a VT emulator. TUI inferiors need a real terminal.

### `gdbforge.open_external_tty()` → `pts_path`

Spawn a terminal that holds a pts open (`tty > file; sleep infinity`) and return `/dev/pts/N`.

### `gdbforge.set_inferior_tty(path|"internal")`

Route program stdin/stdout:

| Backend | Behavior |
|---------|----------|
| **GDB** | Live `-inferior-tty-set` (no session restart) |
| **Delve** | Restarts `dlv exec --tty …` with the same program args |

`"internal"` (or empty) restores the in-app IO pane.

```lua
local pts = gdbforge.open_external_tty()
gdbforge.set_inferior_tty(pts)
```

Env: `GDBFORGE_TERMINAL`, `GDBFORGE_INFERIOR_TTY` (startup path). Closing the external window does **not** auto-rewire — call `set_inferior_tty("internal")`.

**Delve / Go TUIs:** prefer headless + connect instead of mid-session `--tty` restart:

### `gdbforge.spawn_dlv_headless(port [, extra_args...])`

Open an external terminal running headless Delve for `gdbforge.program()` (plus optional extra program args), listening on `port`.

### `gdbforge.dlv_connect(addr)`

Replace the local Delve session with `dlv connect addr` (e.g. `127.0.0.1:2345`). Inferior stdio stays with the headless process / its terminal.

Catalog scripts: `:lua dlv_ext_port [port] [extra args…]` (alias `dlv_port`). Step-by-step for every plugin: [../lua/README.md](../lua/README.md).

---

## Pane draw API (ModeLua widgets)

Available as global `pane` (and mirrored under some hosts):

| Call | Meaning |
|------|---------|
| `pane.clear()` | Clear the cell grid |
| `pane.set_cell(x, y, ch [, fg [, bg]])` | Put character at 0-based cell (optional colors) |
| `pane.size()` → `w, h` | Current pane dimensions |

Used by `:b snake` / `:b tetris` and custom Lua widgets. Prefer `gdbforge.open_buffer("snake")` from `main()` so demos open their pane.

---

## Minimal script example

```lua
-- .gdbforge/lua/hello/hello.lua
local function main(who)
  who = who or "world"
  gdbforge.print("hello, " .. who)
  gdbforge.open_buffer("lua")
end

gdbforge.register("hello", main)
```

```text
:lua hello
:lua hello gdbforge
```

---

## GDB vs Delve cheat sheet

| Concern | GDB | Delve (`gdbforge -g dlv`) |
|---------|-----|---------------------------|
| Entry BP | `break main` (app default) | `break main.main` |
| `gdbforge.gdb` | GDB CLI / MI-friendly cmds | Delve CLI only |
| External TTY | Live `set_inferior_tty` | Restart, or `dlv_port` / `spawn_dlv_headless` + `dlv_connect` |
| TUI bring-up | `terminal_debug`, `gdbserver_tui` | `dlv_port` / `dlv_ext_port` |
| Embedded Linux | `:lua remotegdb` (scp + ssh gdbserver) | — |
| Terminal emulator | `GDBFORGE_TERMINAL=mate-terminal` (shared) | same |

---

## See also

- [USER_GUIDE.md](USER_GUIDE.md) — keys, `:set`, panes, external terminal UX
- [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#external-terminal-stdio-tui-targets) — PTY / tty architecture
- [../lua/README.md](../lua/README.md) — every catalog script, env vars, recipes

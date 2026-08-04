# Lua API (`gdbforge.*`)

User-facing reference for scripting gdbforge.

**Install split:** Framework helpers (`print`, `register`, `spawn`, `pane.*`, …) ship with `luahost`. Debugger bindings (`gdb`, `dlv_connect`, `spawn_dlv_headless`, `set_inferior_tty`, `program`, `current_file`, `current_line`, `stop_file`, `stop_line`) are installed by `cmd/gdbforge` via `gdbforge/luadebug.Install` from `wireUserLuaAPI` — they are present in the debugger app, not in a bare `luahost.New` runtime.
 In-app summary: **`:help`** (Lua section). Architecture/status: [PLUGINS.md](PLUGINS.md). Installable workflows: [../lua/README.md](../lua/README.md).

Scripts are discovered in this order (first basename wins; nested dirs OK):

1. `./.gdbforge/lua/**/*.lua` (project)
2. `~/.gdbforge/lua/**/*.lua` (user home)
3. Embedded catalog shipped with the binary (same tree as [`../lua/`](../lua/))

Each file gets its own Lua VM, loaded **lazily on first** `:lua <basename>` (indexed at startup only — snake/tetris do not run at init). Basename without `.lua` is the command name (e.g. `r5_debug/r5_debug.lua` → `:lua r5_debug`).

```bash
# Optional override — project-local wins over home and embedded:
mkdir -p .gdbforge/lua
cp -r /path/to/custom/r5_debug .gdbforge/lua/
# inside gdbforge:
#   :lua r5_debug
```

Built-in workflows (`remotegdb`, `r5_debug`, …) work with no copy; override only when you need a local edit.
---

## Lifecycle

| Hook | When |
|------|------|
| Top-level script body | Runs once when the script is loaded / `:lua name` first needs it |
| `gdbforge.register("name", fn)` | Exposes `fn` as `:lua name [args…]` |
| `main(...)` (optional convention) | Many catalog scripts define `main` and register it |
| `help()` (optional convention) | `:lua name help` (also `-h` / `--help`) — prints usage to `:b io`; skips `main` |
| ModeLua pane (`:lua snake` / games) | `on_key`, draw via `pane.*`, optional tick — see games under `lua/games/` |

`:lua <func> [args]` calls a registered function; string args are passed as Lua strings.

`:lua <func> help` loads the script VM (if needed) and calls global `help()` when present. Output uses `gdbforge.print` → **`:b io`**. If `help()` is missing, the host prints `no help() for <func>`.

---

## Core helpers

### `gdbforge.print(...)`

Append a line to **`:b io`** (prefixed `[lua]`, coerces args like `print`). Same for automation scripts and ModeLua games — game panes are cell-only (`pane.set_cell`); use `:b io` to read/copy messages.

### `gdbforge.clear()`

Clear the bound Lua pane cell grid (game panes). Does not clear `:b io`.

### `gdbforge.register(name, fn)`

Register `fn` so `:lua name` invokes it. `name` must be a non-empty string; `fn` a Lua function.

### `gdbforge.sleep(seconds)`

Block the Lua VM for `seconds` (number, may be fractional). Used while waiting for ports/probes.
Respects job cancel: **Ctrl-C** during an async `:lua` job aborts sleep with an error.

Automation `:lua <name>` runs **off the UI thread** so the TUI stays responsive. Only one job at a time; a second `:lua` while busy prints a notice. **Ctrl-C** cancels the job: aborts the Lua VM (including after a blocking call returns), interrupts `sleep` / `wait_port`, and kills any in-flight `gdbforge.system` process group. Prefer `gdbforge.system` over `io.popen` for ssh/scp so Ctrl-C can stop them immediately.

ModeLua pane scripts (`on_key` / `on_tick`, e.g. `:lua snake`) run `main()` **on the UI thread** so `open_buffer` cannot deadlock against the tick/draw path.

### `gdbforge.system(cmd)` → `status, output`

Run a shell command (`sh -c`). Returns exit status (number) and combined stdout+stderr (string). On job cancel, the process group is SIGKILL'd and Lua raises an error. Catalog scripts (`remotegdb`, `r5_openamp_jlink`) use this for remote `ssh`/`scp`.

### `gdbforge.lua_dir()`

Absolute directory of the **current** script file (for sidecars: XML, configs next to the `.lua`). Safe to call from registered functions without deadlocking the runtime.

### `gdbforge.program()`

Debuggee path from the current gdbforge session (may be `""` if none). Prefer this over hard-coding binaries in scripts that attach to the session program.

### `gdbforge.current_file()` → `path`

Absolute path of the active CodeWidget file (browse cursor / focused source). Empty string if none.

### `gdbforge.current_line()` → `line`

1-based browse cursor line in the active CodeWidget (or `0` if unknown).

### `gdbforge.stop_file()` → `path`

Source path for the stop PC (`━━▶` from `*stopped`). Empty if no stop location yet.

### `gdbforge.stop_line()` → `line`

1-based stop PC line (`━━▶`), or `0` if unknown.

```lua
local f = gdbforge.stop_file()
local n = gdbforge.stop_line()
-- e.g. :lua gvim / :lua vscode → open at PC when stopped
```

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

Focus a known builtin/buffer (`"code"`, `"gdb"`, `"io"`, `"callstack"`, …), or **create-or-focus** a ModeLua pane when the **calling script** defines `on_key` and/or `on_tick`:

| Call | Result |
|------|--------|
| `open_buffer("snake")` from a pane script | Create a `LuaWidget` named `snake` on first use (adopts this VM), then focus + ModeLua |
| `open_buffer("snake")` again | Focus the same pane |
| `open_buffer("snake1")` from the same script | Create a **second** pane with a new VM loaded from the same script file |
| `open_buffer("gdb")` / unknown name from automation (no pane hooks) | Focus existing, or error — no create |

`:b snake` works only **after** `:lua snake` (or `open_buffer`) has created the pane — it is not listed beforehand. There is no `:b lua` scratch pane.

**Second instance (independent game state):**

```text
:lua snake           " buffer snake (default)
:lua snake snake1    " buffer snake1 — separate VM; :b snake1 to refocus
```

```lua
function main()
  gdbforge.open_buffer("snake")   -- create-or-focus default pane
  -- gdbforge.open_buffer("snake1")  -- optional second instance from script
end
```


---

## Process helpers

### `gdbforge.spawn(argv...)`

Start a **background** PTY process (argv as separate string args or a table). Does **not** steal focus from Code — use for J-Link GDB servers, helpers, etc. Output may appear under `:b exec` if wired.

```lua
gdbforge.spawn("JLinkGDBServer", "-device", "XCZU3CG_R5_0", "-if", "JTAG", "-port", "2331")
```

### `gdbforge.run(argv...)`

Interactive shell / `:!` path — replaces the focused pane with an Exec session. Prefer `spawn` when you must keep Code focused.

### `gdbforge.foreground(argv...)`

Suspend the gdbforge TUI (`tcell` Suspend), run `argv` on the **real** stdin/stdout/stderr, then Resume. Use for terminal editors (`vim`, `nvim`, `less`) that must own the tty until they exit. Blocks the Lua job (and UI) until the process finishes; gdbforge redraws afterward.

```lua
gdbforge.foreground("vim", "+42", "/path/to/file.c")
```

Catalog: `:lua vim` (see [../lua/README.md](../lua/README.md)).

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

Used by dynamic Lua panes (`:lua snake` / `:b snake`, custom scripts). From `main()`, call `gdbforge.open_buffer("name")` so the script creates or focuses its pane. For a second independent game: `:lua snake snake1` (or `open_buffer("snake1")` from Lua).


---

## Minimal script example

```lua
-- .gdbforge/lua/hello/hello.lua
function main(who)
  who = who or "world"
  gdbforge.print("hello, " .. who)  -- appears on :b io
end
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

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

**gdbforge** is a Vim-inspired multi-pane terminal front-end for GDB (and optionally Delve). It keeps the debugger's power in the TTY and adds a workspace for source, console, IO, threads, call stack, and breakpoints — with modes, a `:` command line, mouse/clipboard, and Lua automation.

## Typical use cases

- Debugging **Linux applications** without leaving the terminal
- **Embedded Linux** and board bring-up (source + GDB + process panes together)
- **MCU / cross** workflows where layout and keyboard speed matter
- Teams who want a **Vim-like** flow: normal / insert / command modes, panes, buffers
- **Go** programs via Delve (`-g dlv`), including TUI targets in an external terminal

## Demo

Screencasts (GitHub-hosted). Order: embedded MCU → everyday Linux → dogfooding → **Linux kernel (`kgdb_uart`)**.

**Cortex-R5 / J-Link** — multi-pane UI stepping a deep call stack, with [`lua/r5_debug`](lua/r5_debug) bring-up (`gdbforge.spawn` → JLinkGDBServer → attach). Sample: [`examples/stack_demo.c`](examples/stack_demo.c).

<video src="https://github.com/user-attachments/assets/a5612bb4-c617-401d-b57b-3b8c5543277c" autoplay loop muted playsinline width="100%"></video>

**Linux app** — external terminal print vs the internal IO pane (`:b io`).

<video src="https://github.com/user-attachments/assets/7393b858-e661-44b1-af7e-dbc5d4beef3b" autoplay loop muted playsinline width="100%"></video>

**Debug itself** — gdbforge attached to a live gdbforge session (Go / Delve), stepping its own code.

<video src="https://github.com/user-attachments/assets/6d2466c4-f455-4c7e-a919-62ba330d025b" autoplay loop muted playsinline width="100%"></video>

**Linux kernel (`:lua kgdb_uart`)** — one shared UART + **kdmx**: `:lua kgdb_uart` configures kgdboc, starts kdmx, opens minicom on the console PTY, and breaks into kgdb in about **two seconds** (`target remote` on the gdb PTY). Then `lx-symbols`, a breakpoint on a driver's **read** path, `continue`, and `cat /dev/…` from minicom to hit it — step in `:b gdb`, `continue` back to the shell.

<video src="https://github.com/user-attachments/assets/f3de823b-8cd3-4797-ad83-035ce545926b" autoplay loop muted playsinline width="100%"></video>

<details>
<summary><strong>Kernel demo — setup stages (`kgdb_uart`)</strong></summary>

**Stage 1 — env + one-shot script (host)**

```bash
export GDBFORGE_KGDB_UART=/dev/ttyUSB0
export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux
export GDBFORGE_KGDB_MODULES=/path/to/kernel-source   # for lx-symbols
./bin/gdbforge -g gdb
# then:
:lua kgdb_uart
```

The script claims the UART, sets `kgdboc`, starts **kdmx** (console PTY + gdb PTY), spawns minicom, loads `vmlinux`, sysrq-breaks in, and runs `target remote` — stopped in kgdb in ~2 s.

**Stage 2 — module symbols + breakpoint**

In **`:b gdb`** (after the script returns):

```text
(gdb) lx-symbols /path/to/kernel-source
(gdb) break my_driver_read
(gdb) continue
```

**Stage 3 — trigger from minicom (what the screencast shows)**

In the minicom window on the console PTY:

```text
cat /dev/my_device
```

GDB stops on the driver's read path; debug with `n` / `s` / `c` in gdbforge, then `(gdb) continue` returns to the shell.

Also available: `:lua kgdb_net` (Ethernet), `:lua kgdb_serial` (in-process mux, one UART). Two independent UARTs need no mux — see [docs/KERNEL_KGDB.md](docs/KERNEL_KGDB.md). Catalog: [`lua/README.md`](lua/README.md).

</details>

<details>
<summary><strong>One UART vs two — <code>:serial-switch</code> semi mux</strong></summary>

When only **one** USB serial cable is available, gdbforge can hold `/dev/ttyUSB0` and expose **two PTYs** (console + gdb) via an in-process mux (`:lua kgdb_serial`, `:serial-switch gdb|console`, `:lua kgdb_trigger`). That workflow is **semi-automatic**: you must switch who owns the wire before kgdb stop packets arrive, and **breakpoints triggered from the console while the mux is on the console leg will not reach GDB** (see known limitation in the doc below).

| | Two UARTs (manual) | One UART + kdmx (`kgdb_uart`, **this demo**) | One UART (`kgdb_serial` mux) |
|--|-------------------|---------------------------------------------|------------------------------|
| Console while running | Always on PS0 | minicom on console PTY (kdmx) | minicom on console PTY when mux owner = console |
| GDB while stopped | Always on PS1 | `target remote` on gdb PTY (kdmx) | `target remote` on gdb PTY when owner = gdb |
| `cat` / driver trigger → BP | Works | **Works** (kdmx demux) | Fails unless gdb leg owns UART before trigger |
| Automation | Manual setup | **`:lua kgdb_uart`** (~2 s break-in) | Sysrq-oriented; manual order matters |

Full write-up: **[docs/KERNEL_KGDB.md](docs/KERNEL_KGDB.md)** (Path 1 kdmx, Path 1b in-process mux, Ethernet, recovery, env vars).

</details>

```bash
mkdir -p .gdbforge/lua
cp -r lua/r5_debug .gdbforge/lua/
# then inside gdbforge:  :lua r5_debug
```

More installable workflows (Go/`dlv_ext_port`, embedded/`remotegdb`, **kernel kgdb**, GDB tty, R5, games): [`lua/README.md`](lua/README.md) — install, env vars (`GDBFORGE_TERMINAL=mate-terminal`, `GDBFORGE_KGDB_UART`, …), and how to use each script.

## Host skeleton (`cmd/demo`)

The repo is split so **gdbforge is one app** on a reusable TUI host. A second binary, **`cmd/demo`**, is the minimal showcase of that host — same chrome (modes, `:` cmdline, panes, layouts) and **no GDB/Delve**. Use it as a skeleton when building another product (trader dashboard, ops console, …) on the same framework.

```bash
go build -o bin/demo ./cmd/demo
./bin/demo
```

| Layer | What to reuse |
|-------|----------------|
| **FRAMEWORK** | `termui`, `platform`, `commands`, `ptyx`, `luahost`, … |
| **Skeleton** | `cmd/demo` (+ optional `internal/demo`) — copy / rename as `cmd/<your-app>` |
| **Debugger app** | `cmd/gdbforge` + `internal/gdb` / `dlv` / `gdbforge/*` — do **not** import these from a non-debugger app |

Recipe: wire `TermApp` + command tree + your domain packages; keep debugger imports out of the host. Details and import rules: [`docs/DEPENDENCIES.md`](docs/DEPENDENCIES.md) (`FRAMEWORK vs APP`, `task check-imports`).

## Problems it solves

- Switching constantly between a raw GDB TTY and a separate editor/viewer
- Weak or fixed pane layouts for source, console, and process state
- Limited mouse and clipboard in classic terminal debugger UIs
- Hard-to-discover keys and no in-app manual
- Accidental resume of a running inferior when changing frames/threads

## What you get

- Named layouts (`:layout wide`, `panels`, `default`, `classic`) and splits (`:vs` / `:split`)
- Code (or startup logo), GDB/dlv console, IO, Threads, Call Stack, Breakpoints
- In-app manual: `:help` or `:b help` (full text: [docs/USER_GUIDE.md](docs/USER_GUIDE.md))
- Space to toggle breakpoints; YAML persist under `./.gdbforge/breakpoints.yaml`
- Themability, clipboard/mouse selection, Vim-style modes and focus chords
- Safer while-running BP insert (no surprise continue on frame/thread switches)
- External terminal for TUI inferiors (`:set inferior-tty` / `:lua dlv_ext_port`)
- Lua automation (`gdbforge.*`) — API: [docs/LUA_API.md](docs/LUA_API.md)
- Optional Delve backend: `gdbforge -g dlv ./hello-go` then `:lua dlv_ext_port 1234`

## Install and run (PC)

**Requirements:** Linux (or similar) terminal, [Go](https://go.dev/dl/), `gdb` + `gcc` on `PATH`.

```bash
git clone https://github.com/yairgd/gdbforge.git
cd gdbforge
go build -o bin/gdbforge ./cmd/gdbforge
```

Hello world:

```bash
cat > hello.c <<'HEOF'
#include <stdio.h>
int main(void) { printf("hello, gdbforge\n"); return 0; }
HEOF
gcc -O0 -g -o hello hello.c
./bin/gdbforge ./hello
```

Inside the app: open `:help`, step with `n` / `s` / `c`, quit with Ctrl-D or `:quit`.

```bash
# Delve (Go)
./bin/gdbforge -g dlv ./hello-go

# Pass GDB options after --
./bin/gdbforge -- -nx -x ./board.gdb ./zephyr.elf
```

More demos: `gcc -O0 -g -o stack_demo examples/stack_demo.c && ./bin/gdbforge ./stack_demo`.

## Documentation

| Doc | Contents |
|-----|----------|
| **[docs/USER_GUIDE.md](docs/USER_GUIDE.md)** | Full user manual (same material as `:help`) |
| **[docs/LUA_API.md](docs/LUA_API.md)** | `gdbforge.*` Lua reference for script authors |
| **[docs/PTY_ARCHITECTURE.md](docs/PTY_ARCHITECTURE.md)** | Dual PTY master/slave, GDB vs Delve, `:b io`, external terminal |
| **[docs/DEPENDENCIES.md](docs/DEPENDENCIES.md)** | FRAMEWORK vs APP split; `cmd/demo` as host skeleton |
| **[lua/README.md](lua/README.md)** | Installable Lua workflow catalog |
| **[docs/KERNEL_KGDB.md](docs/KERNEL_KGDB.md)** | Kernel kgdb: two UARTs, kdmx, one-UART mux (`:serial-switch`), Ethernet |
| **[docs/](docs/)** | Architecture, debugger integration, developer guides |

View docs locally: `./docs/serve.sh` → <http://127.0.0.1:8765/>.

## License

MIT License

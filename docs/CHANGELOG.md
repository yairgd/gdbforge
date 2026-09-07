---
title: Changelog
description: Release history for gdbforge — what changed in each tagged version, and what to watch for when upgrading.
---

# Changelog

Release history for gdbforge. Binaries for each version are on the
[GitHub releases page](https://github.com/yairgd/gdbforge/releases); see
[Releasing](RELEASING.md) for how tags drive the builds.

## v1.2.0

Terminal rendering rebuilt on a real xterm emulator, GDB and Delve unified behind one
backend API, and an MVC cleanup of the app core — plus STM32 board scripts and a
documentation overhaul.

### Highlights

- **New terminal pane stack** — the old 1100-line `Viewport` is replaced by a `ScrollDocument` plus an xterm-backed `CompositeTerminal`; input, mouse, scroll, and selection each live in their own module. GDB, IO, and exec panes now share one PTY transport (`WireTTY`) instead of separate console plumbing.
- **Unified backend API** — GDB and Delve sit behind a single semantic `backend.Backend`; controllers call breakpoint, frame, and exec operations instead of formatting MI or Delve CLI strings. Delve runs headless over rpc2.
- **TableWidget** — Breakpoints, Threads, and Call Stack moved onto a shared table widget, off `Viewport`.
- **MVC cleanup** — `DebuggerApp` split into `LayoutShell` and `DebugSession`; `dlvCtl`, `luaCtl`, exec/IO, and search controllers decoupled behind narrow host interfaces; UI events unified on `PostInterrupt` → EventBus → controller handlers.
- **GDB console fixes** — break-while-running and Ctrl-C after the `new-ui mi2` split; Home and End send readline `^A`/`^E` on the prompt line; the view snaps to the bottom on interrupt; wheel and middle-click focus the pane under the pointer.
- **Completion fixes** — `:lua` no longer collides with `:b lua`, the first Tab enters completion mode, and Delve regained Tab completion and multiclient console behavior.
- **Job control and stability** — SIGTSTP is blocked while tcell owns the terminal; `Suspend` and `RunForeground` use `signal.Reset` plus SIGTSTP directly; the terminal is restored when `gdb` or `dlv` is missing at startup; the macOS build is restored.
- **STM32 board catalog** — new `:lua stm32-stlink <board|mcu> [profile]` plus STM32F405 ST-Link and J-Link SWD scripts, and a Zephyr thread-switching fix. Profiles: `baremetal`, `zephyr`, `freertos`.
- **Lua catalog reorganized** — scripts grouped under `lua/mpsoc/`, `lua/stm32/`, `lua/kernel/`, and `lua/embedded/`. `:lua` command names are unchanged.
- **Flow browser** — new `cmd/flowdoc` tool discovers and generates an execution-path catalog, published as a searchable [flow browser](flows/browser.md) on the docs site.
- **Serial backend** — the custom `internal/serial` package is replaced by `go.bug.st/serial`.
- **Documentation** — a [FAQ](FAQ.md), STM32 and MPSoC guides, demo GIFs on the platform pages, public YouTube links, and the documented FreeRTOS profile; several inaccurate probe and kgdb claims corrected.

### Terminal pane refactor at a glance

```text
before:  Viewport (scrollback + ANSI + selection + input, 1137 lines)
after:   ScrollDocument      — scrollback, search, selection
         CompositeTerminal   — xterm emulation (gitpod-io/xterm-go)
         WireTTY             — one PTY transport for GDB / IO / exec
         TableWidget         — Breakpoints / Threads / Call Stack
```

### Upgrading from v1.1.0

- **No breaking CLI changes.** Existing `.gdbforge/` breakpoints and cmdline history remain compatible.
- **Lua script paths moved.** If you copied scripts out of the repo, re-copy from the new locations (`lua/mpsoc/`, `lua/stm32/`, `lua/kernel/`, `lua/embedded/`). Project-local `.gdbforge/lua/` still wins over the embedded catalog, and `:lua` names are unchanged.
- **STM32** — prefer the generic `:lua stm32-stlink <board|mcu> [baremetal|zephyr|freertos]`; per-board aliases (`nucleo_f429zi`, `stm32f405_stlink`, `stm32f405_jlink`) still work. See [STM32 debug](STM32_DEBUG.md).
- **Kernel kgdb** — unchanged from v1.1.0, including patched kdmx (`kdmx -v` → `141210a-gdbforge1`) for the one-UART path. See [Kernel / kgdb](KERNEL_KGDB.md).
- **Delve** — now started headless with rpc2; for Go programs with their own full-screen UI use `:lua dlv_ext_port` (alias `dlv_port`) so program stdio stays in that window.

## v1.1.0

Kernel kgdb automation, a Lua REPL, Assembly UI improvements, and expanded documentation,
building on the v1.0.0 multi-pane GDB/Delve TUI.

### Highlights

- **Kernel and module debugging (kgdb)** — first-class Lua workflows for Linux kernel debug from `:b gdb`:
    - `:lua kgdb_uart` — one shared UART plus kdmx: configures kgdboc, starts kdmx, opens minicom, sysrq break-in, and `target remote` in about two seconds
    - `:lua kgdb_net` — Ethernet kgdb (`target remote` over TCP)
    - `:lua kgdb_serial` and `:lua kgdb_trigger` — in-process UART mux for one-cable setups, with a semi-automatic owner switch
    - Two-UART manual path — console on one cable, GDB on another; no mux and no Lua script required
    - kgdb mode — lighter post-stop refresh on serial, CLI `n`/`s`/`c`, and attach-stack and clean `:q!` fixes
- **Lua REPL pane** — interactive `gdbforge.*` REPL with API help and tab completion.
- **Assembly view** — cgdb-style per-function dumps, `??` windows, and stable scroll; Call Stack click-after-scroll fix; CellStyle rendering instead of generated ANSI.
- **Ctrl-C / Ctrl-Z / Ctrl-D routing** — split into Activity and Confirm routers for clearer interrupt handling.
- **Docs site** — MkDocs site under `docs/` (`./docs/serve.sh`), with a Mermaid lightbox and zoom toolbar.
- **Process hygiene** — spawned children (kdmx, minicom, terminals) are killed when gdbforge exits.
- **Screencasts** — updated README kernel demo (`:lua kgdb_uart`); the two-UART workflow is preserved in [Kernel / kgdb](KERNEL_KGDB.md).

### Kernel kgdb quick start

```bash
export GDBFORGE_KGDB_UART=/dev/ttyUSB0
export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux
export GDBFORGE_KGDB_MODULES=/path/to/kernel-source

./bin/gdbforge -g gdb
# then:
:lua kgdb_uart
# stopped in kgdb — lx-symbols, break, continue, cat /dev/… from minicom
```

Full write-up: [Kernel / kgdb](KERNEL_KGDB.md). Script catalog:
[`lua/README.md`](https://github.com/yairgd/gdbforge/blob/main/lua/README.md).

### Upgrading from v1.0.0

- No breaking CLI changes. Existing `.gdbforge/` breakpoints and cmdline history are compatible.
- Kernel workflows are optional Lua scripts — copy `lua/kgdb_*` into `.gdbforge/lua/` or use the embedded catalog.
- For `:lua kgdb_uart`, use patched kdmx (`kdmx -v` → `141210a-gdbforge1`). Build from [agent-proxy](https://git.kernel.org/pub/scm/utils/kernel/kgdb/agent-proxy.git) at commit `468fe4c` and apply [`tools/kdmx-gdbforge.patch`](https://github.com/yairgd/gdbforge/blob/main/tools/kdmx-gdbforge.patch) — see [building kdmx](KERNEL_KGDB.md#building-kdmx-tested-setup).

## v1.0.0

First tagged 1.0 of gdbforge: a Vim-inspired multi-pane terminal front-end for GDB and Delve.

### Highlights

- **Multi-pane workspace** — Code, GDB/dlv console, IO, Threads, Call Stack, Breakpoints, and Assembly.
- **Layouts** — `:layout wide`, `panels`, `default`, and `classic`; splits (`:vs` and `:split`); `:only`.
- **Vim-like UX** — Normal, Insert, Command, Search, Completion, and Lua modes; the `:` cmdline; focus chords.
- **GDB and Delve** — `-g gdb|dlv`, a shared session, safer breakpoint insert while running, and conditional breakpoints.
- **Mouse and clipboard** — selection, middle-click paste, and double-click on a status name to copy the full path.
- **Persistence** — breakpoints and cmdline history under `.gdbforge/`.
- **Lua automation** — embedded catalog plus project and home scripts; `:lua` jobs cancellable with Ctrl-C; games, remotegdb, and Cortex-R5 J-Link bring-up.
- **AI / MCP** — same-process tools on the live session (`:AI`).
- **Host skeleton** — `cmd/demo` reuses the TUI framework without a debugger.
- **Docs and release** — in-app `:help`, the docs site, and tag-driven multi-arch binaries.

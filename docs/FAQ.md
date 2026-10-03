---
title: gdbforge FAQ — cgdb and GDB TUI Comparison, Setup, Targets
description: Answers to common gdbforge setup questions — how it compares to cgdb and the GDB TUI, where program I/O goes, supported targets and probes, installation, and Delve.
---

# FAQ

Questions that usually come up when setting up gdbforge for a new target. Full manuals: [USER_GUIDE.md](USER_GUIDE.md) · [OVERVIEW.md](OVERVIEW.md).

---

## How is gdbforge different from cgdb?

Both are keyboard-driven terminal front-ends that put source, a GDB console, and auxiliary views on one screen. [cgdb](https://github.com/cgdb/cgdb) is mature C/ncurses software built around GDB only. gdbforge is written in Go with a recursive split tree (`:vs`, `:split`, `:layout`), a dedicated pane for the inferior's stdio, GDB **and** Delve backends (`-g gdb|dlv`), and Lua scripts for probe and target bring-up.

Going the other way, cgdb is more mature, is packaged by most distributions, and is a much smaller program to trust. Feature-by-feature table: [OVERVIEW.md — Comparison to cgdb and gdb TUI](OVERVIEW.md#comparison-to-cgdb-and-gdb-tui).

Coming from cgdb, the everyday equivalents are `:b gdb` for the console, `:edit` / `:b <file>` for source, Space to toggle a breakpoint, `n` / `s` / `c` / `f` for run control, and `:layout wide` for a multi-pane workspace.

---

## How is gdbforge different from GDB's built-in TUI?

GDB's TUI (`layout src`, `Ctrl-X A`) draws source, assembly, and register windows inside GDB itself, from a fixed set of layouts sized with `winheight`. It has no arbitrary splits, no separate window for the program's own stdin/stdout, and no breakpoint, thread, or call-stack list views.

gdbforge stays outside GDB and drives it over **MI2 on a second UI channel** (`new-ui mi2`), so the source view and list panes follow `*stopped` records while the console remains a normal interactive GDB session. Details: [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md).

Two things the TUI has going for it: it is already inside every GDB install, so there is nothing to download, and it has a register window, which gdbforge does not have a pane for yet (`:gdb info registers` prints to the console instead). Assembly is covered on both sides — gdbforge has an assembly pane (`:b asm`, `:layout <name> asm`), though only under the GDB backend.

---

## Should I use the internal IO pane or an external terminal?

`:b io` is the default and needs no setup; it suits line-oriented output and stdin. Switch the inferior to a real terminal emulator (`:set inferior-tty`, or `:lua terminal_debug`) when the program is a full-screen curses/TUI application, expects a proper VT, or floods stdout. Under `gdbserver`, the program's stdio belongs to gdbserver's own terminal.

Both options, with the underlying GDB `-inferior-tty-set` and Delve `--tty` behavior, are covered in [EMBEDDED_LINUX_DEBUG.md — Program I/O](EMBEDDED_LINUX_DEBUG.md#program-io--internal-vs-external) and [PTY_ARCHITECTURE.md](PTY_ARCHITECTURE.md).

---

## Which targets and probes are supported?

Anything GDB itself can reach; gdbforge adds Lua bring-up scripts for the common cases:

| Target | Transport | Guide |
|--------|-----------|-------|
| STM32 / Cortex-M firmware | ST-Link + OpenOCD, or J-Link over SWD | [STM32_DEBUG.md](STM32_DEBUG.md) |
| Zynq UltraScale+ Cortex-A53 / Cortex-R5 | J-Link GDB Server, or OpenOCD + Digilent JTAG-HS2 | [MPSOC_DEBUG.md](MPSOC_DEBUG.md) |
| Applications on an embedded Linux board | `gdbserver` over SSH, `target remote` | [EMBEDDED_LINUX_DEBUG.md](EMBEDDED_LINUX_DEBUG.md) |
| Linux kernel and loadable modules | kgdb over UART or Ethernet | [KERNEL_KGDB.md](KERNEL_KGDB.md) |

Probe support comes from OpenOCD, the J-Link GDB Server, or gdbserver — gdbforge orchestrates them rather than implementing its own probe driver.

---

## Which STM32 profile should I use — baremetal, Zephyr, or FreeRTOS?

The ST-Link scripts take the profile as their last argument (`:lua nucleo_f429zi zephyr`):

| Profile | Use for | Effect |
|---------|---------|--------|
| `baremetal` | Firmware without RTOS thread awareness (default) | No `-rtos` on the OpenOCD target |
| `zephyr` | Zephyr applications | `-rtos Zephyr`, plus `dir` for `$ZEPHYR_BASE` and the app; needs `CONFIG_DEBUG_THREAD_INFO=y` |
| `freertos` | FreeRTOS firmware | `-rtos FreeRTOS` |

See [STM32_DEBUG.md](STM32_DEBUG.md) for the per-board commands and environment variables.

---

## Can I debug Go programs?

Yes, with Delve: `gdbforge -g dlv ./your-program`. The panes and key bindings are the same as under GDB, with the differences listed in [USER_GUIDE.md](USER_GUIDE.md) (Tab completion, `Ctrl-C`, and `f` mapping to `stepout`). One pane is missing: the assembly view is GDB-only, because the Delve backend reports no assembly support.

For a Go program with its own full-screen UI, run `:lua dlv_ext_port` (alias `dlv_port`): Delve starts headless in a separate terminal window, keeps the program's stdio there, and gdbforge connects to it. Background: [DEBUGGER_INTEGRATION.md — Delve backend](DEBUGGER_INTEGRATION.md#delve-backend-peer-of-gdb).

---

## Where are breakpoints stored between sessions?

In `./.gdbforge/breakpoints.yaml`, relative to the directory gdbforge was started from — normally your build directory. They are written on quit and restored on the next start from the same directory, so run gdbforge from your build tree to keep them with the project.

---

## Is gdbforge ready for daily use?

The GDB integration, panes, layouts, breakpoint persistence, and the embedded and kernel Lua workflows all work today and are used for real debugging, and there are tagged [releases](https://github.com/yairgd/gdbforge/releases) with prebuilt binaries.

Set expectations accordingly, though: it is maintained by a small number of contributors, there is only ever one tab, there are no register or memory panes (use `:gdb info registers` and GDB's `x` in the console), the Lua API can still change between releases, and Delve trails GDB — notably no assembly pane. Per-component state and planned work: [ROADMAP.md](ROADMAP.md).

---

## How do I install it?

Download a prebuilt binary from the [latest release](https://github.com/yairgd/gdbforge/releases/latest) (Linux and macOS, amd64 and arm64), or `go install github.com/yairgd/gdbforge/cmd/gdbforge@latest`, or build from a clone. Step-by-step, including checksum verification and a first debugging session: [Install](README.md#install).

However you install it, the Lua workflow catalog under [`lua/`](https://github.com/yairgd/gdbforge/tree/main/lua) and the helper shell scripts (`gdbforge --list-scripts`) are embedded in the binary, so `:lua remotegdb`, `:lua nucleo_f429zi`, `:lua r5_baremetal_jlink` and `:lua kgdb_uart` work without a checkout. Copy a script into `./.gdbforge/lua/` only to customise it — project-local scripts take precedence ([details](LUA_API.md)).

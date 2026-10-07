---
title: gdbforge — Terminal Debugger for Embedded, Kernel and Linux Targets
description: Debug MCU firmware, embedded Linux apps, the Linux kernel and desktop programs from one terminal. gdbforge is a keyboard-driven, multi-pane GDB front-end that also automates target bring-up with OpenOCD, J-Link, gdbserver and kgdb.
---

# gdbforge

**Debug MCU firmware, embedded Linux apps, the Linux kernel and desktop programs from one
terminal — one command from target to breakpoint.**
{ .gf-tagline }

GDB stays the debugger. gdbforge is a keyboard-driven, multi-pane front-end around it —
source, the real GDB console, program I/O, threads, call stack and breakpoints on one
screen — and one `:lua` command brings the target up: it starts OpenOCD, the J-Link GDB
Server, `gdbserver` over SSH, or kgdb, and attaches.

**Built for:** Linux desktop applications · applications on embedded Linux boards ·
the Linux kernel and its modules · firmware on any MCU that OpenOCD or the J-Link GDB
Server can reach (bundled bring-up scripts cover STM32 and Zynq MPSoC Cortex-A53/R5).

[Install](#install){ .md-button .md-button--primary }
[First debugging session](#your-first-debugging-session){ .md-button }
[User guide](USER_GUIDE.md){ .md-button }

MIT licensed · Linux and macOS, `amd64` and `arm64` ·
[source on GitHub](https://github.com/yairgd/gdbforge)

![gdbforge debugging Cortex-R5 firmware over J-Link: source, call stack, threads, and breakpoint panes updating while stepping](media/gdbforge-demo-r5.gif)

**Cortex-R5 bare-metal firmware over J-Link.** `:lua r5_baremetal_jlink` spawns
JLinkGDBServer, attaches and loads the ELF; then a deep call stack is stepped with the
source, call stack and breakpoint panes updating together.
[Watch on YouTube](https://www.youtube.com/watch?v=jbS5SE7Xu3g) ·
[Zynq MPSoC guide](MPSOC_DEBUG.md) ·
samples: [`examples/stack_demo.c`](https://github.com/yairgd/gdbforge/blob/main/examples/stack_demo.c) (bare metal),
[`examples/zephyr_cortex_r5/`](https://github.com/yairgd/gdbforge/tree/main/examples/zephyr_cortex_r5) (Zephyr, thread-aware) ·
[more demos below](#demos-by-use-case)

---

## Why gdbforge

| | **gdbforge** | **cgdb** | **GDB TUI** | **Vitis / Eclipse** | **VS Code + Cortex-Debug** |
|---|---|---|---|---|---|
| Terminal-native | Yes | Yes | Yes | No — desktop GUI | No — runs inside VS Code |
| Works over SSH | Yes | Yes | Yes | — <!-- TODO verify --> | — <!-- TODO verify: VS Code Remote-SSH --> |
| One-command target bring-up | `:lua` scripts start OpenOCD, J-Link GDB Server, `gdbserver` or kgdb, then attach | No | No | — <!-- TODO verify: launch configurations --> | — <!-- TODO verify: launch.json servertype --> |
| Real GDB console | Yes — `:b gdb` is the GDB session | Yes | Yes — it is GDB | — <!-- TODO verify --> | — <!-- TODO verify: debug console with -exec --> |
| Program I/O separate from the debugger | `:b io` pane, or an external terminal | No — shares the terminal | No — shares the terminal | — <!-- TODO verify --> | — <!-- TODO verify --> |
| Breakpoint, thread and call-stack views | Panes that refresh on every stop | Via GDB commands | No | Yes | Yes |
| Register and memory views | Not yet — GDB commands in the console | Via GDB commands | Register window | Yes | Yes |
| kgdb workflow | `:lua kgdb_uart` — one UART with kdmx; also two UARTs or Ethernet | Manual GDB commands | Manual GDB commands | — <!-- TODO verify --> | — <!-- TODO verify --> |
| Scriptable | Lua (`gdbforge.*`); API not frozen yet | Limited | GDB Python, no UI hooks | — <!-- TODO verify --> | — <!-- TODO verify --> |

If you want register and peripheral views today, an IDE is the better fit; if you want
plain GDB in a terminal, cgdb and the GDB TUI are smaller and more mature. gdbforge is for
terminal users who also want target bring-up, program I/O and list panes in one
workspace. More detail: [FAQ](FAQ.md#how-is-gdbforge-different-from-cgdb) ·
[full comparison](OVERVIEW.md#comparison-to-cgdb-and-gdb-tui).

---

## Supported workflows

| You want to debug | gdbforge gives you | Guide |
|---|---|---|
| A **Linux application** on your own machine | `gdbforge ./prog`; output in `:b io` or an external terminal | [Linux applications](EMBEDDED_LINUX_DEBUG.md) |
| An application on a **remote or embedded Linux board** | `:lua remotegdb` — scp the binary, start `gdbserver` over SSH, `target remote` | [Linux applications](EMBEDDED_LINUX_DEBUG.md) |
| **STM32 firmware** — bare-metal, Zephyr, or FreeRTOS | `:lua nucleo_f429zi` — ST-Link + OpenOCD or J-Link over SWD, stop at `main` | [STM32 and Zephyr](STM32_DEBUG.md) |
| **Zynq UltraScale+ MPSoC** Cortex-A53 / Cortex-R5 | `:lua r5_baremetal_jlink` — J-Link GDB Server or OpenOCD + Digilent HS2 | [Zynq MPSoC](MPSOC_DEBUG.md) |
| The **Linux kernel** or a loadable module | `:lua kgdb_uart` — kgdb over one UART with kdmx, two UARTs, or Ethernet | [Kernel and kgdb](KERNEL_KGDB.md) |
| A **Go program**, including one with its own TUI | `gdbforge -g dlv ./prog`, `:lua dlv_ext_port` for a separate terminal | [FAQ — Go and Delve](FAQ.md#can-i-debug-go-programs) |

Probe and transport support comes from OpenOCD, the J-Link GDB Server, or `gdbserver`.
gdbforge orchestrates those tools rather than implementing its own probe driver.

---

## Install

**Requirements:** Linux or macOS with a UTF-8 terminal, and `gdb` (or `dlv` for Go) on
your `PATH`. The hello-world below also needs `gcc`.

=== "Download a release binary"

    Prebuilt binaries are published for Linux and macOS on `amd64` and `arm64`. Replace
    the version with the [latest release](https://github.com/yairgd/gdbforge/releases/latest)
    and pick the file matching your platform.

    ```bash
    VERSION=v1.3.0
    OS=linux          # or: darwin
    ARCH=amd64        # or: arm64

    curl -fL -o gdbforge \
      "https://github.com/yairgd/gdbforge/releases/download/${VERSION}/gdbforge-${VERSION}-${OS}-${ARCH}"
    chmod +x gdbforge
    sudo mv gdbforge /usr/local/bin/
    ```

    Each binary ships a matching `.sha256` file:

    ```bash
    curl -fLO "https://github.com/yairgd/gdbforge/releases/download/${VERSION}/gdbforge-${VERSION}-${OS}-${ARCH}.sha256"
    sha256sum -c "gdbforge-${VERSION}-${OS}-${ARCH}.sha256"
    ```

=== "go install"

    ```bash
    go install github.com/yairgd/gdbforge/cmd/gdbforge@latest
    ```

    Installs into `$(go env GOPATH)/bin`. Binaries built this way report their version as
    `dev`, because the version string is stamped at release build time.

=== "Build from source"

    ```bash
    git clone https://github.com/yairgd/gdbforge.git
    cd gdbforge
    go build -o bin/gdbforge ./cmd/gdbforge
    ```

    Building requires the Go version in [`go.mod`](https://github.com/yairgd/gdbforge/blob/main/go.mod)
    or newer. Use this if you want to edit the bundled
    [Lua workflow scripts](https://github.com/yairgd/gdbforge/tree/main/lua) locally.

The Lua workflow catalog and the helper shell scripts (`gdbforge --list-scripts`) are
embedded in the binary, so `:lua r5_baremetal_jlink`, `:lua remotegdb`, `:lua kgdb_uart`
and the rest work from any of the three installs — no checkout required. Project-local
scripts in `./.gdbforge/lua/` override the embedded ones when you want to customise a
workflow ([details](LUA_API.md)).

Check the install:

```bash
gdbforge -version
gdbforge --help
```

---

## Your first debugging session

Build a program with debug symbols and open it:

```bash
cat > hello.c <<'EOF'
#include <stdio.h>

static int add(int a, int b) { return a + b; }

int main(void) {
    int sum = add(2, 3);
    printf("hello, gdbforge: %d\n", sum);
    return 0;
}
EOF

gcc -O0 -g -o hello hello.c
gdbforge ./hello
```

gdbforge opens with the source pane and the GDB console (`:b gdb`) alongside it. From
there:

1. **Set a breakpoint.** Move the cursor to the `int sum = add(2, 3);` line with the arrow
   keys and press <kbd>Space</kbd>. The line gets a red marker and the breakpoint appears
   in the Breakpoints pane. (Equivalent: type `break main` in `:b gdb`.)
2. **Start the program.** Type `:gdb run`. Execution stops on your line and `━━▶` marks the
   program counter. Use `:gdb run` — not <kbd>c</kbd> — for the first start; <kbd>c</kbd>
   is `continue`, which GDB rejects until the program is running.
3. **Step.** <kbd>s</kbd> steps into `add`, <kbd>n</kbd> steps over, <kbd>f</kbd> finishes
   the current frame, <kbd>c</kbd> continues. The Call Stack and Threads panes refresh at
   every stop.
4. **Inspect.** Press <kbd>i</kbd> to focus the GDB console and type `print sum` — or any
   other GDB command — then <kbd>Esc</kbd> to return to normal mode.
5. **See the program's output.** `:b io` shows the `printf` output in its own pane.
6. **Get help or leave.** `:help` opens the in-app manual; `:quit` or <kbd>Ctrl-D</kbd>
   exits.

Your breakpoints are written to `./.gdbforge/breakpoints.yaml` on exit and restored the
next time you start gdbforge from the same directory.

Full key and command reference: [User guide](USER_GUIDE.md) · common setup questions:
[FAQ](FAQ.md).

---

## Demos by use case

Every screencast below is a recording of a real session. The Cortex-R5 demo is at the
[top of this page](#gdbforge).

### Embedded and bare-metal firmware

**STM32 Nucleo F429ZI — bare-metal, then Zephyr-aware.** `:lua nucleo_f429zi baremetal`
debugs the application on the board over ST-Link and OpenOCD; `:lua nucleo_f429zi zephyr`
re-attaches with OpenOCD's `-rtos Zephyr` so Zephyr threads show up in `info threads` and
in the Threads pane.
[Watch on YouTube](https://www.youtube.com/watch?v=_RAPSW77HcQ) ·
[STM32 and Zephyr guide](STM32_DEBUG.md)

![gdbforge debugging a Zephyr application on an STM32 Nucleo F429ZI board over ST-Link](media/gdbforge-demo-stm32-nucleo-f429zi.gif){ loading=lazy }

### Linux applications

**Program I/O — internal pane versus external terminal.** The same program run two ways:
once with its stdout in the built-in `:b io` pane, and once attached to a real terminal
emulator, which is what full-screen curses programs need.
[Watch on YouTube](https://www.youtube.com/watch?v=Eya_zs4M1Cg) ·
[Linux application guide](EMBEDDED_LINUX_DEBUG.md)

![gdbforge debugging a Linux application, comparing the internal IO pane with an external terminal](media/gdbforge-demo-linux-app.gif){ loading=lazy }

### Linux kernel and modules

**Kernel kgdb over a single UART.** `:lua kgdb_uart` configures `kgdboc`, starts kdmx to
split the one serial line into a console PTY and a gdb PTY, opens minicom on the console,
and breaks into kgdb in about two seconds. Then `lx-symbols`, a breakpoint on a driver's
read path, and `cat /dev/…` from the console to hit it.
[Watch on YouTube](https://www.youtube.com/watch?v=6eEIxdKQTWY) ·
[Kernel and kgdb guide](KERNEL_KGDB.md)

![gdbforge breaking into the Linux kernel with kgdb over a single UART using kdmx](media/gdbforge-demo-kernel-kgdb.gif){ loading=lazy }

**Kernel kgdb with two UARTs.** When the board has a separate console and kgdb cable there
is no mux and no bring-up script — the simplest and most reliable kgdb setup.
[Watch on YouTube](https://www.youtube.com/watch?v=yhOO8CEh1LA) ·
[Kernel guide — two UARTs](KERNEL_KGDB.md#path-0--two-uarts-manual-recommended)

### Go and Delve

**gdbforge debugging itself.** A gdbforge process attached to another live gdbforge
session through Delve (`-g dlv`), stepping its own Go code — the same panes and keys as
under GDB.
[Watch on YouTube](https://www.youtube.com/watch?v=tDNT1MQSQoE) ·
[Delve backend details](DEBUGGER_INTEGRATION.md#delve-backend-peer-of-gdb)

![gdbforge attached to its own running session through Delve, stepping its own Go code](media/gdbforge-debug-itself.gif){ loading=lazy }

---

## Project status

gdbforge is released and versioned — see the
[releases page](https://github.com/yairgd/gdbforge/releases) and the
[changelog](CHANGELOG.md). The debugging workflows on this page are used for real work,
but the project is maintained by a small number of contributors and parts of it are still
moving. A realistic summary:

| Area | State |
|------|-------|
| GDB backend — MI2 on a dedicated channel, console, source sync, breakpoints, threads, call stack | Works; the most exercised path |
| Delve backend (`-g dlv`) | Works for everyday Go debugging, but is not a full peer of GDB: the assembly pane is disabled, and some commands differ (see [user guide](USER_GUIDE.md)) |
| Program I/O — `:b io` pane and external terminal | Works |
| Split tree, layouts, modes, `:` command line with completion, mouse | Works (provided by [termforge](https://yairgd.github.io/termforge/)) |
| Assembly pane (`:layout <name> asm`, `:vs asm`) | Works under GDB only |
| Breakpoint persistence (`./.gdbforge/breakpoints.yaml`) | Works |
| Lua workflow scripts and the `gdbforge.*` API | Works — 25 `gdbforge.*`/`pane.*` functions and about 30 bundled workflow scripts, all embedded in the binary. The API is **not** versioned or frozen and may change between releases |
| Tabs | One tab only — no tab bar, no `:tabnew` |
| Register and memory panes | Not implemented — use `:gdb info registers` and GDB's `x` in the console |
| Native OpenOCD backend (telnet/TCL) | Not implemented — OpenOCD is launched as an external GDB server by the Lua scripts |

Planned work and known gaps: [roadmap](ROADMAP.md).

---

More: [internals and contributing](INTERNALS.md) ·
built on [termforge](https://yairgd.github.io/termforge/) ·
[releases](https://github.com/yairgd/gdbforge/releases) ·
[issues](https://github.com/yairgd/gdbforge/issues) ·
[CONTRIBUTING.md](https://github.com/yairgd/gdbforge/blob/main/CONTRIBUTING.md)

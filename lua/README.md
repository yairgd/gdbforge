---
description: gdbforge Lua debug scripts — embedded Linux apps, MPSoC ZynqMP A53/R5, STM32 bare-metal, Linux kgdb kernel debug, and Delve workflows for a terminal debugger UI.
meta:
  - name: keywords
    content: GDB debugger, terminal debug UI, embedded Linux debugger, embedded debugger, MPSoC debug, STM32 GDB, Linux kernel kgdb, remotegdb, gdbforge, cgdb alternative
---

# gdbforge Lua scripts

**API:** [docs/LUA_API.md](../docs/LUA_API.md) · **In-app:** `:help` · **User guide:** [docs/USER_GUIDE.md](../docs/USER_GUIDE.md)

Installable workflows under `lua/`. Copy what you need into `.gdbforge/lua/`, then `:lua <name>` (Tab completes). Command name = Lua file basename.

---

## Platform sections

| Section | Path | Targets |
|---------|------|---------|
| **[Embedded Linux](embedded/README.md)** | `lua/embedded/` | Board apps (`remotegdb`), I/O (`external_tty`, `terminal_debug`), local `gdbserver_tui` |
| **[MPSoC](mpsoc/README.md)** | `lua/mpsoc/` | Zynq UltraScale+ — Cortex-A53 + Cortex-R5 (J-Link / OpenOCD) |
| **[STM32](stm32/README.md)** | `lua/stm32/` | STM32F405 bare-metal (J-Link SWD / ST-Link OpenOCD) |
| **[Kernel](kernel/README.md)** | `lua/kernel/` | Linux kgdb — UART, Ethernet, module symbols |

Each section README has install steps, script catalog, and env vars. Published docs (meta tags, Open Graph): [EMBEDDED_LINUX_DEBUG.md](../docs/EMBEDDED_LINUX_DEBUG.md), [MPSOC_DEBUG.md](../docs/MPSOC_DEBUG.md), [STM32_DEBUG.md](../docs/STM32_DEBUG.md), [KERNEL_KGDB.md](../docs/KERNEL_KGDB.md).

---

## Install

```bash
mkdir -p .gdbforge/lua

# MPSoC (pick one or both CPU folders)
cp -r lua/mpsoc/cortex_r5 .gdbforge/lua/

# STM32F405
cp -r lua/stm32/stm32f405 .gdbforge/lua/

# Kernel kgdb
cp -r lua/kernel/kgdb_common lua/kernel/kgdb_kdmx .gdbforge/lua/

# Embedded Linux app on board
cp -r lua/embedded/remotegdb .gdbforge/lua/

# External terminal I/O (local TUI apps)
cp -r lua/embedded/terminal_debug .gdbforge/lua/

# Host utilities (Delve, editors, games)
cp -r lua/dlv_ext_port .gdbforge/lua/
```

---

## Shared environment variables

| Variable | Meaning |
|----------|---------|
| **`GDBFORGE_TERMINAL`** | Terminal emulator for spawn_terminal workflows |
| `GDBFORGE_INFERIOR_TTY` | Force inferior pts at startup |

Per-platform env vars are in each section README.

---

## Host utilities (lua/ root)

| Directory | `:lua` | Backend | Purpose |
|-----------|--------|---------|---------|
| [`dlv_ext_port/`](dlv_ext_port/) | `dlv_ext_port` | Delve | Go TUI: headless dlv in external terminal |
| [`dlv_port/`](dlv_port/) | `dlv_port` | Delve | Alias of `dlv_ext_port` |
| [`gvim/`](gvim/) | `gvim` | — | Open Code source in gVim |
| [`vim/`](vim/) | `vim` | — | Open Code source in terminal vim |
| [`vscode/`](vscode/) | `vscode` | — | Open Code source in VS Code |
| [`games/snake/`](games/snake/) | `snake` | — | Demo pane |
| [`games/tetris/`](games/tetris/) | `tetris` | — | Demo pane |

---

## Directory layout

```text
lua/
├── README.md              ← you are here
├── embedded/
│   ├── README.md
│   ├── remotegdb/         # board: :lua remotegdb
│   ├── terminal_debug/    # external tty + load
│   ├── external_tty/      # external tty only
│   └── gdbserver_tui/     # local gdbserver
├── mpsoc/
│   ├── README.md
│   ├── cortex_a53/        # A53 J-Link + OpenOCD
│   └── cortex_r5/         # R5 J-Link + OpenOCD + OpenAMP
├── stm32/
│   ├── README.md
│   └── stm32f405/         # J-Link + ST-Link
├── kernel/
│   ├── README.md
│   └── kgdb_*/            # kgdb workflows
├── dlv_ext_port/
└── …
```

After install, copied folders live flat under `.gdbforge/lua/` (e.g. `.gdbforge/lua/remotegdb/`, `.gdbforge/lua/kgdb_kdmx/`).

---

## Adding a new workflow

1. Create `lua/<section>/<name>/<name>.lua` with `function main(...)`.
2. Add a row to the section README catalog.
3. Keep the script self-contained — users edit env defaults at the top.

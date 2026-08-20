---
description: STM32 bare-metal debug with gdbforge — STM32F405 J-Link SWD and ST-Link OpenOCD GDB workflows. Flash, halt, and break at main from a terminal debugger UI.
meta:
  - name: keywords
    content: STM32 debugger, STM32F405 GDB, ST-Link debug, J-Link STM32 SWD, Cortex-M4 flash debug, OpenOCD STM32, embedded GDB terminal, bare-metal STM32, gdbforge
---

# STM32 debug (STM32F405)

**gdbforge** is a Vim-inspired **GDB terminal UI** for **STM32 bare-metal** development. Lua scripts under [`lua/stm32/stm32f405/`](https://github.com/yairgd/gdbforge/tree/main/lua/stm32/stm32f405) spawn **J-Link GDB Server (SWD)** or **ST-Link + OpenOCD**, connect GDB, reset, flash your ELF, and break at `main` — about 100 lines each, easy to copy and edit.

## Quick start — J-Link SWD

```bash
mkdir -p .gdbforge/lua
cp -r lua/stm32/stm32f405 .gdbforge/lua/
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
./bin/gdbforge ./build/firmware.elf
:lua stm32f405_jlink
```

## Quick start — ST-Link + OpenOCD

```bash
cp -r lua/stm32/stm32f405 .gdbforge/lua/
./bin/gdbforge ./build/firmware.elf
:lua stm32f405_stlink
```

Requires `openocd` on PATH. Sidecar config: `stm32f405_openocd.cfg` (edit adapter speed or target for other F4 parts).

## Script catalog

| `:lua` | Probe | Purpose |
|--------|-------|---------|
| `stm32f405_jlink` | J-Link SWD | Spawn JLinkGDBServer → load → break main |
| `stm32f405_stlink` | ST-Link + OpenOCD | Spawn openocd → load → break main |

## Environment variables

| Variable | Default |
|----------|---------|
| `GDBFORGE_JLINK` | Path to `JLinkGDBServer` |
| `GDBFORGE_JLINK_DEVICE` | `STM32F405RG` |
| `GDBFORGE_JLINK_PORT` | `2334` |
| `GDBFORGE_OPENOCD` | `openocd` |
| `GDBFORGE_OPENOCD_CFG` | `stm32f405_openocd.cfg` |
| `GDBFORGE_OPENOCD_PORT` | `3333` |

For other STM32F4 MCUs, change `GDBFORGE_JLINK_DEVICE` or edit the `DEVICE` line at the top of `stm32f405_jlink.lua`.

See also: [Lua catalog — STM32](https://github.com/yairgd/gdbforge/blob/main/lua/stm32/README.md) · [User Guide — Lua](USER_GUIDE.md)

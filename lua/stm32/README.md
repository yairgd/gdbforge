---
description: STM32F405 bare-metal debug with gdbforge — J-Link SWD and ST-Link OpenOCD GDB workflows. Flash ELF, halt, break at main from a terminal debugger UI.
meta:
  - name: keywords
    content: STM32 debugger, STM32F405 GDB, ST-Link debug, J-Link STM32 SWD, Cortex-M4 flash debug, OpenOCD STM32, embedded GDB terminal, bare-metal STM32, gdbforge
---

# STM32 MCU debugging (STM32F405)

Self-contained Lua scripts for **STM32F405** bare-metal debug over **J-Link (SWD)** or **ST-Link (OpenOCD)**. Full guide: **[STM32_DEBUG.md](../../docs/STM32_DEBUG.md)**.

## Quick start

```bash
mkdir -p .gdbforge/lua
cp -r lua/stm32/stm32f405 .gdbforge/lua/
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
./bin/gdbforge ./build/firmware.elf
:lua stm32f405_jlink
```

ST-Link (needs `openocd` on PATH):

```bash
cp -r lua/stm32/stm32f405 .gdbforge/lua/
./bin/gdbforge ./build/firmware.elf
:lua stm32f405_stlink
```

## Who is this for?

Developers who want a **GDB terminal UI** for STM32 bare-metal — flash, halt, load ELF, break at `main` — without leaving the keyboard-driven gdbforge panes. Scripts are short (~100 lines); edit defaults at the top of each file.

## Script catalog

| `:lua` | Probe | Purpose |
|--------|-------|---------|
| `stm32f405_jlink` | J-Link SWD | Spawn JLinkGDBServer → load → break main |
| `stm32f405_stlink` | ST-Link + OpenOCD | Spawn openocd → load → break main |

Sidecar: `stm32f405_openocd.cfg` — edit adapter speed or target for other F4 parts.

## Common searches

### How to debug STM32F405 with GDB?

Install gdbforge, copy `lua/stm32/stm32f405/` to `.gdbforge/lua/`, open your `.elf`, run `:lua stm32f405_jlink` (J-Link) or `:lua stm32f405_stlink` (ST-Link).

### ST-Link vs J-Link for STM32?

**J-Link** — set `GDBFORGE_JLINK_DEVICE` (default `STM32F405RG`). Uses SWD, not JTAG. **ST-Link** — uses OpenOCD with the bundled `stm32f405_openocd.cfg`; works with ST-Link V2/V3 on-board or external.

### STM32 Cortex-M4 flash and debug from terminal

Both scripts: `monitor reset halt`, `load`, `break main`. No custom target XML needed (unlike Zynq R5).

### Other STM32F4 parts (F407, F411, …)?

Change `GDBFORGE_JLINK_DEVICE` or edit the `DEVICE` line at the top of `stm32f405_jlink.lua`. For OpenOCD, adjust `stm32f405_openocd.cfg` (e.g. `target/stm32f4x.cfg` covers most F4).

## Environment variables

| Variable | Script | Default |
|----------|--------|---------|
| `GDBFORGE_JLINK` | jlink | `/opt/.../JLinkGDBServer` |
| `GDBFORGE_JLINK_DEVICE` | jlink | `STM32F405RG` |
| `GDBFORGE_JLINK_PORT` | jlink | `2334` |
| `GDBFORGE_OPENOCD` | stlink | `openocd` |
| `GDBFORGE_OPENOCD_CFG` | stlink | `stm32f405_openocd.cfg` |
| `GDBFORGE_OPENOCD_PORT` | stlink | `3333` |

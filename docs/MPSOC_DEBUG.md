---
description: MPSoC debug with gdbforge — automate Zynq UltraScale+ Cortex-A53 and Cortex-R5 GDB sessions over J-Link or Digilent OpenOCD from a terminal debugger UI.
meta:
  - name: keywords
    content: MPSoC debug, ZynqMP debugger, Zynq UltraScale+ GDB, Xilinx embedded debug, Cortex-A53 debugger, Cortex-R5 debugger, J-Link ZynqMP, OpenOCD Zynq, bare-metal MPSoC, OpenAMP remoteproc, gdbforge
---

# MPSoC debug (Zynq UltraScale+)

**gdbforge** is a Vim-inspired **GDB terminal UI** for **Xilinx Zynq UltraScale+ MPSoC** development. Lua scripts under [`lua/mpsoc/`](https://github.com/yairgd/gdbforge/tree/main/lua/mpsoc) spawn **J-Link GDB Server** or **OpenOCD + Digilent JTAG-HS2**, attach with `target remote`, load your ELF, and set breakpoints — the Code pane stays usable while the probe runs in the background.

Scripts live in two folders (copy what you need into `.gdbforge/lua/`):

| Directory | CPU | Workflows |
|-----------|-----|-----------|
| [`lua/mpsoc/cortex_a53/`](https://github.com/yairgd/gdbforge/tree/main/lua/mpsoc/cortex_a53) | Cortex-A53 | Bare-metal, Linux kernel attach |
| [`lua/mpsoc/cortex_r5/`](https://github.com/yairgd/gdbforge/tree/main/lua/mpsoc/cortex_r5) | Cortex-R5 | Bare-metal, OpenAMP / remoteproc |

## Quick start — Cortex-R5 + J-Link

```bash
mkdir -p .gdbforge/lua
cp -r lua/mpsoc/cortex_r5 .gdbforge/lua/
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
./bin/gdbforge ./your_app.elf
:lua r5_baremetal_jlink
```

## Script catalog

| `:lua` | Probe | Purpose |
|--------|-------|---------|
| `a53_baremetal_jlink` | J-Link | A53 bare-metal load + break main |
| `a53_baremetal_openocd_digilent` | OpenOCD | A53 bare-metal (Digilent HS2) |
| `a53_kernel_jlink` | J-Link | A53 Linux kernel (kgdb attach) |
| `a53_kernel_openocd_digilent` | OpenOCD | A53 Linux kernel (Digilent HS2) |
| `r5_baremetal_jlink` | J-Link | R5 bare-metal load + break main |
| `r5_baremetal_openocd_digilent` | OpenOCD | R5 bare-metal (Digilent HS2) |
| `r5_openamp_jlink` | J-Link | R5 OpenAMP attach + load |
| `r5_openamp_openocd_digilent` | OpenOCD | R5 OpenAMP (Digilent HS2) |

## Environment variables

| Variable | Default / meaning |
|----------|-------------------|
| `GDBFORGE_JLINK` | Path to `JLinkGDBServer` |
| `GDBFORGE_JLINK_CHIP` | Chip prefix (`XCZU3CG`) |
| `GDBFORGE_JLINK_DEVICE` | Full device override |
| `GDBFORGE_JLINK_PORT` | GDB port (`2334`) |
| `GDBFORGE_R5_CORE` | RPU core `0`/`1` |
| `GDBFORGE_A53_CORE` | APU core `0`–`3` |
| `GDBFORGE_OPENOCD` | `openocd` on PATH |
| `GDBFORGE_OPENOCD_PORT` | GDB port (`3333`) |

Edit defaults at the top of any script, or export before running. Each script implements `help()` — run `:lua <name>` and check the Lua pane output.

See also: [Lua catalog — MPSoC](https://github.com/yairgd/gdbforge/blob/main/lua/mpsoc/README.md) · [User Guide — Lua](USER_GUIDE.md)

---
description: MPSoC debug with gdbforge — Zynq UltraScale+ Cortex-A53 and Cortex-R5 GDB workflows via J-Link or Digilent OpenOCD. One-command bare-metal load, OpenAMP attach, and A53 kernel debug from a terminal debugger UI.
meta:
  - name: keywords
    content: MPSoC debug, ZynqMP debugger, Zynq UltraScale+ GDB, Xilinx embedded debug, Cortex-A53 debugger, Cortex-R5 debugger, J-Link ZynqMP, OpenOCD Zynq, bare-metal MPSoC, OpenAMP remoteproc, gdbforge
---

# Zynq MPSoC debugging (Cortex-A53 + Cortex-R5)

gdbforge Lua scripts for **Xilinx Zynq UltraScale+ MPSoC** debug: bare-metal and kernel on **Cortex-A53**, bare-metal and OpenAMP on **Cortex-R5**, using **SEGGER J-Link** or **Digilent JTAG-HS2 + OpenOCD**. Full guide on the docs site: **[MPSOC_DEBUG.md](../../docs/MPSOC_DEBUG.md)**.

## Quick start

```bash
mkdir -p .gdbforge/lua
cp -r lua/mpsoc/cortex_r5 .gdbforge/lua/
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
./bin/gdbforge ./your_app.elf
:lua r5_baremetal_jlink
```

## Who is this for?

Embedded developers debugging **ZynqMP** firmware or Linux on the A53 application cores, or **R5 lock-step / OpenAMP** workloads. Scripts spawn the probe GDB server in the background so the gdbforge Code pane stays usable.

## Script catalog

| `:lua` | Directory | Probe | Purpose |
|--------|-----------|-------|---------|
| `a53_baremetal_jlink` | `cortex_a53/` | J-Link | A53 bare-metal load + break main |
| `a53_baremetal_openocd_digilent` | `cortex_a53/` | OpenOCD | A53 bare-metal (Digilent HS2) |
| `a53_kernel_jlink` | `cortex_a53/` | J-Link | A53 Linux kernel (kgdb attach) |
| `a53_kernel_openocd_digilent` | `cortex_a53/` | OpenOCD | A53 Linux kernel (Digilent HS2) |
| `r5_baremetal_jlink` | `cortex_r5/` | J-Link | R5 bare-metal load + break main |
| `r5_baremetal_openocd_digilent` | `cortex_r5/` | OpenOCD | R5 bare-metal (Digilent HS2) |
| `r5_openamp_jlink` | `cortex_r5/` | J-Link | R5 OpenAMP attach + load |
| `r5_openamp_openocd_digilent` | `cortex_r5/` | OpenOCD | R5 OpenAMP (Digilent HS2) |

Install either CPU folder (or both):

```bash
cp -r lua/mpsoc/cortex_a53 .gdbforge/lua/
cp -r lua/mpsoc/cortex_r5 .gdbforge/lua/
```

## Common searches

### How do I debug Cortex-R5 with GDB on ZynqMP?

Copy `lua/mpsoc/cortex_r5/` into `.gdbforge/lua/`, start gdbforge with your ELF, run `:lua r5_baremetal_jlink`. J-Link spawns automatically; GDB attaches with `target remote`.

### J-Link vs OpenOCD on Xilinx MPSoC?

**J-Link** — set `GDBFORGE_JLINK` and optionally `GDBFORGE_JLINK_CHIP` (default `XCZU3CG`). **OpenOCD + Digilent HS2** — set `GDBFORGE_OPENOCD`; cfg and `r5_target.xml` ship beside the scripts.

### How to debug Zynq UltraScale+ bare metal?

Use `a53_baremetal_jlink` or `r5_baremetal_jlink` depending on the core. Select RPU core with `GDBFORGE_R5_CORE=0|1`, A53 with `GDBFORGE_A53_CORE=0..3`.

### OpenAMP / remoteproc debug on R5?

After Linux has loaded the R5 firmware via remoteproc, use `:lua r5_openamp_jlink ./firmware.elf` (or the OpenOCD variant).

## Key environment variables

| Variable | Used by | Default / meaning |
|----------|---------|-------------------|
| `GDBFORGE_JLINK` | J-Link scripts | Path to `JLinkGDBServer` |
| `GDBFORGE_JLINK_CHIP` | J-Link | Chip prefix (`XCZU3CG`) |
| `GDBFORGE_JLINK_DEVICE` | J-Link | Full device override |
| `GDBFORGE_JLINK_PORT` | J-Link | GDB port (`2334`) |
| `GDBFORGE_R5_CORE` | R5 | `0`/`1` or `R0`/`R1` |
| `GDBFORGE_A53_CORE` | A53 | `0`–`3` or `A0`–`A3` |
| `GDBFORGE_OPENOCD` | OpenOCD | `openocd` on PATH |
| `GDBFORGE_OPENOCD_CFG` | OpenOCD | Sidecar `.cfg` in script dir |
| `GDBFORGE_OPENOCD_PORT` | OpenOCD | GDB port (`3333`) |
| `GDBFORGE_TDESC` | R5 | `r5_target.xml` beside script |

Each script has `:lua <name>` then check in-app help via the script's `help()` function. Edit env defaults at the top of any `.lua` file to match your board.

# Zephyr on Cortex-R5 — gdbforge debugging demo

Three threads (`main`, `worker0`, `worker1`), a shared struct behind a mutex, and a
`run_iteration` → `compute_sample` → `accumulate` → `fib` call chain. Built `-Og` with
thread awareness on, so breakpoints, stepping, backtraces and the Threads pane all work.

## Requirements

- Zynq UltraScale+ MPSoC RPU, Cortex-R5 in split mode. Built against upstream `kv260_r5`.
- Runs from ATCM at `0x0`. Hard 64K ceiling; text+rodata must stay under 32K (see `prj.conf`).
- An FSBL must already have run `psu_init`, or the image loads and does nothing.
  No FSBL? `../../scripts/zynqmp-park-el3.sh -p <platform>/hw/psu_init.tcl`, from a shell,
  with no gdbforge session open.
- J-Link for the JTAG console, or a Digilent JTAG-HS2 for the OpenOCD path.
- `-S uart0-console` needs the R5 to own `uart0` at `0xFF000000`. Linux on the APU normally
  does, which is why the JTAG console exists.

## Quick start

`./build.sh` handles Zephyr, the toolchain and both consoles. Run `./build.sh` with no
arguments for the full option list.

```bash
cd examples/zephyr_cortex_r5

./build.sh init --use ~/zephyrproject/zephyr   # adopt a Zephyr you already have
./build.sh init --download                     # or fetch v4.3.0 into ~/zephyrproject

./build.sh build -c jtag                       # console over SEGGER RTT / J-Link
./build.sh build -c uart0                      # console over PS UART0, 115200 8N1
./build.sh debug                               # install Lua workflows, start gdbforge
```

`init` records the tree in `.zephyr-base`, so nothing after it needs `ZEPHYR_BASE` in the
environment. `--use` never writes to your Zephyr or installs a second SDK; it only checks
that the tree and a toolchain are there.

## Install Zephyr and the toolchain by hand

What `init --download` runs, if you would rather do it yourself:

```bash
pip install west
west init -m https://github.com/zephyrproject-rtos/zephyr --mr v4.3.0 ~/zephyrproject
cd ~/zephyrproject && west update && west packages pip --install
west sdk install -t arm-zephyr-eabi
```

## Build by hand

```bash
export ZEPHYR_BASE=~/zephyrproject/zephyr
cd examples/zephyr_cortex_r5

west build -b kv260_r5 . -S jtag-console     # console over SEGGER RTT / J-Link
west build -b kv260_r5 . -S uart0-console    # console over PS UART0, 115200 8N1
```

Pick exactly one. A bare `west build -b kv260_r5 .` also works and leaves the board default
console on `uart1`. Add `-p always` when switching snippets: a snippet stays in the CMake
cache, so building one over the other merges both. `./build.sh` does this for you.

`-O0` (`-- -DCONFIG_NO_OPTIMIZATIONS=y`) does not fit — it overflows the TCM by about 20K,
because text+rodata crosses 32K and the MPU rounds the ROM region up to 64K. See `prj.conf`.

## Debug by hand

Once, to install the Lua workflows:

```bash
mkdir -p .gdbforge/lua && cp -r ../../lua/mpsoc/cortex_r5 .gdbforge/lua/
```

Then:

```bash
export ZEPHYR_BASE=~/zephyrproject/zephyr   # gdb needs it for kernel sources
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
gdbforge build/zephyr/zephyr.elf
```

```
:lua r5_baremetal_jlink zephyr              # J-Link
:lua r5_baremetal_openocd_digilent zephyr   # OpenOCD + JTAG-HS2
```

Either script halts the core, zeroes ATCM for ECC, loads, sets `$pc` to `0x0` and breaks on
`main`. Append `help` to either for the full set of environment variables and caveats.

## JTAG console

RTT auto-search will not find the control block — the R5's TCM is outside the ranges J-Link
scans. Read it from the map file and hand it over:

```bash
./build.sh rtt                               # e.g. 0x8080
gdbforge --run-script rtt.sh                 # separate terminal
```

```
monitor exec SetRTTAddr 0x8080
continue
```

`continue` is not optional: RTT only flows while the core is executing.

## Notes

- OpenOCD is the accurate path for Zephyr threads. SEGGER's stock `RTOSPlugin_Zephyr` decodes
  Cortex-M exception frames, so under J-Link thread names are fine but the registers and
  backtrace of any non-running thread are not. Details in `r5_common.lua`.
- `src/gic_warm_restart.c` retires interrupts left active by a reload that skips the system
  reset (`GDBFORGE_JLINK_NORESET=1`). Without it a stale active TTC interrupt pins the GIC
  running priority and no tick is ever delivered, so `k_sleep()` never returns. No-op on a
  cold boot, and it needs no changes to Zephyr.
- `boot_stage` in `src/main.c` accumulates one nibble per init level, for breakpoints that
  have to fire before `main` — try `break mk_pk1_first`.

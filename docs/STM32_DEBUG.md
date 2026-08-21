---
description: STM32 bare-metal debug with gdbforge — Nucleo F429ZI and STM32F405 (boards 1–2 in an extensible catalog). ST-Link OpenOCD and J-Link SWD; more boards under lua/stm32/<board>/.
meta:
  - name: keywords
    content: STM32 debugger, Nucleo F429ZI, STM32F405, ST-Link debug, J-Link STM32 SWD, Cortex-M4 flash debug, OpenOCD STM32, Zephyr GDB, embedded GDB terminal, bare-metal STM32, gdbforge
---

# STM32 debug

**gdbforge** is a Vim-inspired **GDB terminal UI** for **STM32 bare-metal** (and Zephyr) development. Lua scripts under [`lua/stm32/`](https://github.com/yairgd/gdbforge/tree/main/lua/stm32) spawn **ST-Link + OpenOCD** or **J-Link GDB Server**, attach GDB, flash your ELF, and stop at `main`.

## Getting started (order of operations)

Follow these steps once, then repeat from step 5 for each debug session.

1. **Host tools** — install **OpenOCD** (ST-Link boards) and/or **J-Link software** (board 2 J-Link path). See [OpenOCD version and install](#openocd-version-and-install) below.
2. **USB / udev** — plug in the board; ensure ST-Link is visible (`lsusb`). On Linux, install [OpenOCD udev rules](https://github.com/openocd-org/openocd/blob/master/contrib/60-openocd.rules) if permission denied.
3. **gdbforge** — build or install gdbforge; verify `openocd --version` on PATH.
4. **Board scripts** — copy the board folder into your project (no gdbforge rebuild):

   ```bash
   mkdir -p .gdbforge/lua
   cp -r /path/to/gdbforge/lua/stm32/nucleo_f429zi .gdbforge/lua/   # board 1
   cp -r /path/to/gdbforge/lua/stm32/stm32f405 .gdbforge/lua/       # board 2
   ```

5. **Build firmware** — Zephyr `west build`, Makefile, CMake, etc.; note path to `zephyr.elf` / `firmware.elf`.
6. **Environment** — export board-specific vars (Zephyr `ZEPHYR_BASE`, OpenOCD cfg paths — see board sections below).
7. **Launch gdbforge** with your ELF from the build directory, e.g. `gdbforge ./zephyr/zephyr.elf`
8. **Connect probe** — `:lua nucleo_f429zi` (or `stm32f405_stlink` / `stm32f405_jlink`).
9. **Debug** — stopped at `main` in `:b gdb`; `continue` to run; OpenOCD log at `/tmp/gdbforge-openocd.log` (or `GDBFORGE_OPENOCD_LOG`); `:b code` for source.

OpenOCD is started detached (like kdmx): the `:lua` job finishes and Ctrl-C no longer tears down OpenOCD. It is stopped when you run `:lua nucleo_f429zi` again or when gdbforge exits.

---

## OpenOCD version and install

ST-Link scripts (`nucleo_f429zi`, `stm32f405_stlink`) run **`openocd`** on the host. gdbforge does not bundle OpenOCD.

### Version

| | |
|--|--|
| **Tested with** | **OpenOCD 0.12.0** (current stable; works with Nucleo F429ZI + Zephyr `-rtos Zephyr`) |
| **Minimum** | **0.12.0** recommended; Zephyr thread awareness (`info threads`) needs **> 0.11.0** |
| **Check** | `openocd --version` → should print `Open On-Chip Debugger 0.12.x` |

Scripts path (for `[find board/…]` in cfg): usually **`/usr/share/openocd/scripts`** on Linux.

### Download & install

**Official project**

- Home: [https://openocd.org/](https://openocd.org/)
- Getting OpenOCD: [https://openocd.org/pages/getting-openocd/](https://openocd.org/pages/getting-openocd/)
- Source / releases: [https://github.com/openocd-org/openocd/releases](https://github.com/openocd-org/openocd/releases)

**Linux package managers** (fastest if version ≥ 0.12)

```bash
# Debian / Ubuntu
sudo apt update && sudo apt install openocd

# Fedora
sudo dnf install openocd

# Gentoo
sudo emerge -av openocd
```

**Zephyr SDK** (includes OpenOCD + scripts + udev rules)

- Download: [https://github.com/zephyrproject-rtos/sdk-ng/releases](https://github.com/zephyrproject-rtos/sdk-ng/releases)
- After install, OpenOCD is under the SDK sysroot; udev rules:

  ```bash
  sudo cp $ZEPHYR_SDK_INSTALL_DIR/sysroots/x86_64-pokysdk-linux/usr/share/openocd/contrib/60-openocd.rules /etc/udev/rules.d/
  sudo udevadm control --reload
  ```

  Point gdbforge at SDK OpenOCD if not on PATH:

  ```bash
  export GDBFORGE_OPENOCD=$ZEPHYR_SDK_INSTALL_DIR/sysroots/x86_64-pokysdk-linux/usr/bin/openocd
  ```

**Build from source** (latest git)

```bash
git clone https://github.com/openocd-org/openocd.git
cd openocd
./bootstrap && ./configure --enable-stlink && make -j$(nproc)
sudo make install
```

**Override binary** (any install location):

```bash
export GDBFORGE_OPENOCD=/usr/bin/openocd    # default: openocd on PATH
```

---

## Board catalog (extensible)

Scripts are grouped by board folder: **`lua/stm32/<board>/`**. The list below is the **first two entries**; new boards are added as new folders (same layout). Board number is documentation order only — not a version or priority flag.

| # | Board folder | MCU / kit | `:lua` scripts | Probe |
|---|--------------|-----------|----------------|-------|
| **1** | [`nucleo_f429zi/`](../lua/stm32/nucleo_f429zi/) | STM32F429ZI (Nucleo-F429ZI) | `nucleo_f429zi`, `nucleo_f429zi_stlink` | On-board ST-Link + OpenOCD |
| **2** | [`stm32f405/`](../lua/stm32/stm32f405/) | STM32F405 | `stm32f405_stlink`, `stm32f405_jlink` | ST-Link + OpenOCD or J-Link SWD |
| _3+_ | `<board>/` | _(future)_ | `<board>_stlink`, … | per board |

**Adding board #3:** create `lua/stm32/<board>/` with at least one `*.lua` (`:lua` command = basename) and optional `*_openocd.cfg`. Copy [`nucleo_f429zi/`](../lua/stm32/nucleo_f429zi/) for ST-Link + Zephyr/OpenOCD, or [`stm32f405/`](../lua/stm32/stm32f405/) for a generic F4 + J-Link variant. Update this table and [`lua/stm32/README.md`](../lua/stm32/README.md).

Install scripts into the project (no gdbforge rebuild):

```bash
mkdir -p .gdbforge/lua
cp -r lua/stm32/nucleo_f429zi .gdbforge/lua/   # board 1
cp -r lua/stm32/stm32f405 .gdbforge/lua/       # board 2
```

---

## 1 · Nucleo F429ZI (ST-Link + OpenOCD)

Typical **Zephyr** app on Nucleo-F429ZI.

### Command line

Example (west build dir = project folder; ELF at `zephyr/zephyr.elf`):

```bash
cd ~/alyn/alyn/game-controller/nucleo_f429zi
mkdir -p .gdbforge/lua
cp -r /path/to/gdbforge/lua/stm32/nucleo_f429zi .gdbforge/lua/

export ZEPHYR_BASE=~/alyn/zephyr-3.4.99/zephyr

gdbforge ./zephyr/zephyr.elf
```

Inside gdbforge:

```
:lua nucleo_f429zi zephyr
```

Only **`ZEPHYR_BASE`** is required for Zephyr. OpenOCD board cfg and GDB app dir are derived from `ZEPHYR_BASE` and your `$PWD`. Profile **`zephyr`** enables OpenOCD **`-rtos Zephyr`** (needs `CONFIG_DEBUG_THREAD_INFO=y`). Stops at **`main`**. Check threads: `info threads` in `:b gdb`.

For bare metal (no thread awareness): `:lua nucleo_f429zi baremetal` — ignores `ZEPHYR_BASE` for RTOS.

### ST-Link script flow

OpenOCD (background) → wait port **3333** → `dir` Zephyr paths → `target remote` → `break main` → `continue` (stops at **main**).

Optional env: `GDBFORGE_OPENOCD`, `GDBFORGE_OPENOCD_PORT` (default `3333`).

---

## 2 · STM32F405 (ST-Link + OpenOCD)

Same ST-Link / OpenOCD pattern as board 1.

```bash
mkdir -p .gdbforge/lua
cp -r lua/stm32/stm32f405 .gdbforge/lua/
export GDBFORGE_OPENOCD_SCRIPTS=/usr/share/openocd/scripts   # if bundled cfg [find …] fails
gdbforge ./build/firmware.elf
:lua stm32f405_stlink
```

Then `:b gdb`: `continue` to run (already loaded and stopped at `main` if script finished OK).

---

## 2 · STM32F405 (J-Link SWD)

```bash
cp -r lua/stm32/stm32f405 .gdbforge/lua/
export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer
gdbforge ./build/firmware.elf
:lua stm32f405_jlink
```

Spawns J-Link, `target remote`, `load`, `break main`.

---

## Script catalog (all boards)

| # | Board | `:lua` | Probe | Purpose |
|---|-------|--------|-------|---------|
| 1 | Nucleo F429ZI | `nucleo_f429zi` | ST-Link + OpenOCD | OpenOCD + GDB attach (Zephyr-friendly) |
| 1 | | `nucleo_f429zi_stlink` | _(alias)_ | same |
| 2 | STM32F405 | `stm32f405_stlink` | ST-Link + OpenOCD | same OpenOCD flow as board 1 |
| 2 | | `stm32f405_jlink` | J-Link SWD | J-Link + load + break main |

---

## Zephyr awareness (GDB, not gdbforge core)

gdbforge does **not** embed Zephyr-specific logic. Use standard GDB setup:

| Need | How |
|------|-----|
| Source paths | `export ZEPHYR_BASE=~/alyn/zephyr-3.4.99/zephyr` then `:lua … zephyr` — script runs `dir $ZEPHYR_BASE` and `dir $PWD` |
| Cross-GDB | `gdbforge -d …/arm-zephyr-eabi-gdb -- ./zephyr/zephyr.elf` |
| OpenOCD board cfg | derived from `$ZEPHYR_BASE/boards/arm/<board>/support/openocd.cfg` |
| **`info threads` (Zephyr RTOS)** | `CONFIG_DEBUG_THREAD_INFO=y` **and** `:lua … zephyr` **and** `ZEPHYR_BASE` set |

---

## Environment variables (ST-Link scripts)

| Variable | Default | Notes |
|----------|---------|-------|
| `GDBFORGE_OPENOCD` | `openocd` | see [OpenOCD version and install](#openocd-version-and-install) |
| `GDBFORGE_OPENOCD_PORT` | `3333` | |
| `ZEPHYR_BASE` | _(unset)_ | **Required for profile zephyr** — kernel tree; OpenOCD cfg derived automatically |
| `GDBFORGE_STM32_BOARD` | _(unset)_ | Default board for `:lua stm32_stlink` when only profile is passed |

J-Link (`stm32f405_jlink` only): `GDBFORGE_JLINK`, `GDBFORGE_JLINK_DEVICE` (`STM32F405RG`), `GDBFORGE_JLINK_PORT` (`2334`).

See also: [Lua catalog — STM32](../lua/stm32/README.md) · [User Guide — Lua](USER_GUIDE.md)

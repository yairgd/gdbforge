---
description: Linux kernel debug with gdbforge kgdb — UART kdmx, Ethernet kgdboe, in-process serial mux, lx-symbols, and module breakpoints from a GDB terminal UI.
meta:
  - name: keywords
    content: Linux kernel debugger, kgdb GDB, kgdb UART, kdmx kernel debug, kgdboe Ethernet, kernel module debug, lx-symbols, embedded Linux driver debug, gdbforge
---

# Linux kernel debugging (kgdb)

gdbforge **kgdb** Lua workflows for **Linux kernel and loadable module** debug. Full guide: **[KERNEL_KGDB.md](../../docs/KERNEL_KGDB.md)**

## Quick start

```bash
mkdir -p .gdbforge/lua
cp -r lua/kernel/kgdb_common lua/kernel/kgdb_kdmx .gdbforge/lua/
export GDBFORGE_KGDB_UART=/dev/ttyUSB0
export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux
./bin/gdbforge -g gdb /path/to/vmlinux
:lua kgdb_kdmx
```

## Who is this for?

Kernel and driver developers who want a **cgdb-like GDB UI** for kgdb — breakpoints in modules, `lx-symbols`, console in minicom, break-in in ~2 s with `:lua kgdb_kdmx`.

## Script catalog

| `:lua` | Mux? | Purpose |
|--------|------|---------|
| `kgdb_kdmx` | kdmx | **Recommended** — one-shot UART workflow |
| `kgdb_uart` | kdmx | UART + external kdmx (step-by-step) |
| `kgdb_serial` | in-process | One UART, semi-auto mux |
| `kgdb_trigger` | in-process | Break-in after `kgdb_serial` |
| `kgdb_net` | — | Ethernet kgdb (`target remote` TCP) |
| `kgdb_setup` | — | kgdboc setup only (SSH) |
| `kgdb_detach` | — | Cleanup / disconnect |
| `kgdb_load_module` | — | Reload module symbols |
| `kgdb_common` | — | Shared helpers (not a workflow) |

Install the scripts you need (always include `kgdb_common`):

```bash
cp -r lua/kernel/kgdb_common lua/kernel/kgdb_kdmx .gdbforge/lua/
```

## Common searches

### Linux kernel debugger with GDB terminal UI

Use gdbforge with `:lua kgdb_kdmx` — orchestrates kdmx, minicom, sysrq break-in, and `target remote` in one command. See [KERNEL_KGDB.md](../../docs/KERNEL_KGDB.md).

### kgdb UART debug one serial cable

`:lua kgdb_kdmx` (kdmx split) or `:lua kgdb_serial` + `:lua kgdb_trigger` (in-process mux). Two separate UARTs need no Lua — see KERNEL_KGDB Path 0.

### kgdboe / Ethernet kernel debug

```bash
cp -r lua/kernel/kgdb_common lua/kernel/kgdb_net .gdbforge/lua/
export GDBFORGE_REMOTE_HOST=192.168.20.50
:lua kgdb_net
```

### Debug Linux kernel modules with lx-symbols

Set `GDBFORGE_KGDB_VMLINUX` and `GDBFORGE_KGDB_MODULES` (kernel build tree). After attach, run `lx-symbols` in GDB (scripts source kernel `vmlinux-gdb.py` when configured).

## Key environment variables

| Variable | Meaning |
|----------|---------|
| `GDBFORGE_KGDB_UART` | Host serial device |
| `GDBFORGE_KGDB_VMLINUX` | Path to `vmlinux` |
| `GDBFORGE_KGDB_MODULES` | Kernel build tree for symbols |
| `GDBFORGE_REMOTE_HOST` | Board IP (net kgdb) |
| `GDBFORGE_KGDB_PORT` | TCP port (default `6443`) |
| `GDBFORGE_TERMINAL` | Terminal for minicom / ssh windows |

See [KERNEL_KGDB.md](../../docs/KERNEL_KGDB.md) for kdmx patching, two-UART wiring, and full env reference.

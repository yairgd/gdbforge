-- Cortex-A53 / Digilent HS2 OpenOCD kernel attach (running Linux, no load).
-- Install: cp -r lua/cortex_a53 lua/kgdb_common .gdbforge/lua/
-- Usage:   :lua a53_kernel_openocd_digilent
--          :lua a53_kernel_openocd_digilent /path/to/vmlinux
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_OPENOCD_*      same as a53_baremetal_openocd_digilent
--   GDBFORGE_KGDB_VMLINUX   host vmlinux (required)
--   GDBFORGE_KGDB_MODULES   kernel tree for lx-symbols
--   GDBFORGE_KGDB_SCRIPTS   kernel tree with vmlinux-gdb.py

local C = dofile(gdbforge.lua_dir() .. "/a53_common.lua")

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/a53_openocd_digilent.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"

function help()
  gdbforge.print("a53_kernel_openocd_digilent — OpenOCD attach, vmlinux, lx-symbols (no load)")
  gdbforge.print("Usage: :lua a53_kernel_openocd_digilent [vmlinux]")
  C.kernel_prereq_help()
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_A53_CORE=0")
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux")
  gdbforge.print("  export GDBFORGE_KGDB_SCRIPTS=/path/to/kernel-source")
end

function main(vmlinux_arg)
  local core, bad = C.a53_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_A53_CORE must be 0|1|2|3|A0..A3 (got " .. tostring(bad) .. ")")
    return
  end

  local vmlinux = C.resolve_vmlinux(vmlinux_arg)
  if vmlinux == "" then
    gdbforge.print("ERROR: set vmlinux — :lua a53_kernel_openocd_digilent /path/to/vmlinux")
    gdbforge.print("       or export GDBFORGE_KGDB_VMLINUX")
    return
  end

  local st = gdbforge.system("test -f " .. C.shell_quote(vmlinux))
  if st ~= 0 then
    gdbforge.print("ERROR: vmlinux not found: " .. vmlinux)
    return
  end

  local kgdb = C.load_kgdb_common()
  if not kgdb then
    return
  end

  st = gdbforge.system("test -f " .. C.shell_quote(CFG))
  if st ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. CFG)
    return
  end

  local modules = C.env("GDBFORGE_KGDB_MODULES", kgdb.kgdb_kernel_tree())
  local scripts = C.env("GDBFORGE_KGDB_SCRIPTS", kgdb.kgdb_kernel_tree())

  gdbforge.print("A53 core: " .. core)
  gdbforge.print("vmlinux: " .. vmlinux)
  gdbforge.print("cfg: " .. CFG)

  C.stop_openocd()
  gdbforge.print("starting openocd (Digilent HS2 attach, A53 #" .. core .. ") …")
  gdbforge.spawn(OPENOCD, "-c", "set A53_CORE " .. core, "-f", CFG)

  if not C.wait_probe(PORT, 20, C.openocd_alive, "openocd") then
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("file " .. vmlinux)
  gdbforge.gdb("set architecture aarch64")
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")

  kgdb.load_symbols({
    vmlinux = vmlinux,
    modules_dir = modules,
    scripts = scripts,
  })

  gdbforge.print("a53_kernel_openocd_digilent done — set kernel breakpoints in :b gdb")
  gdbforge.print("  module symbols: :lua kgdb_load_module [name]")
end

-- Cortex-A53 / J-Link kernel attach (running Linux, no load).
-- Install: cp -r lua/cortex_a53 lua/kgdb_common .gdbforge/lua/
-- Usage:   :lua a53_kernel_jlink
--          :lua a53_kernel_jlink /path/to/vmlinux
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_JLINK_*        same as a53_baremetal_jlink
--   GDBFORGE_KGDB_VMLINUX   host vmlinux (required)
--   GDBFORGE_KGDB_MODULES   kernel tree for lx-symbols
--   GDBFORGE_KGDB_SCRIPTS   kernel tree with vmlinux-gdb.py

local C = dofile(gdbforge.lua_dir() .. "/a53_common.lua")

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local CHIP = os.getenv("GDBFORGE_JLINK_CHIP") or "XCZU3CG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"

function help()
  local core = C.a53_core() or 0
  local device = C.jlink_device(CHIP, core)
  gdbforge.print("a53_kernel_jlink — J-Link attach (-noir -noreset), vmlinux, lx-symbols")
  gdbforge.print("Usage: :lua a53_kernel_jlink [vmlinux]")
  C.kernel_prereq_help()
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_A53_CORE=0")
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. device)
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
    gdbforge.print("ERROR: set vmlinux — :lua a53_kernel_jlink /path/to/vmlinux")
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

  local device = C.jlink_device(CHIP, core)
  local modules = C.env("GDBFORGE_KGDB_MODULES", kgdb.kgdb_kernel_tree())
  local scripts = C.env("GDBFORGE_KGDB_SCRIPTS", kgdb.kgdb_kernel_tree())

  gdbforge.print("A53 core: " .. core .. "  device: " .. device)
  gdbforge.print("vmlinux: " .. vmlinux)

  C.stop_jlink()
  gdbforge.print("starting JLinkGDBServer (attach: -noreset -noir) …")
  gdbforge.spawn(
    JLINK,
    "-device", device,
    "-if", "JTAG",
    "-speed", "4000",
    "-port", PORT,
    "-noreset",
    "-noir"
  )

  if not C.wait_probe(PORT, 15, C.jlink_alive, "JLinkGDBServer") then
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

  gdbforge.print("a53_kernel_jlink done — set kernel breakpoints in :b gdb, then continue")
  gdbforge.print("  module symbols: :lua kgdb_load_module [name]")
end

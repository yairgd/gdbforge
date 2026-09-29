-- Cortex-A53 / Digilent HS2 OpenOCD bare-metal bring-up.
-- Install: cp -r lua/mpsoc/cortex_a53 .gdbforge/lua/
-- Usage:   :lua a53_baremetal_openocd_digilent
--
-- Prerequisite: the core must already be at EL3, which a normally booted board is not by the
-- time you can halt it. scripts/zynqmp-park-el3.sh gets it there. It is run by hand from a
-- shell, before and outside gdbforge, with no debug session open — this script only points
-- at it. See a53_common.PARK_SCRIPT and that script's --help.
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_OPENOCD        path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG    OpenOCD config (default: script dir a53_openocd_digilent.cfg)
--   GDBFORGE_OPENOCD_PORT   GDB listen port (default 3333)
--   GDBFORGE_A53_ENTRY      where to resume after load (default &_boot)

local C = dofile(gdbforge.lua_dir() .. "/a53_common.lua")

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/a53_openocd_digilent.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"

function help()
  gdbforge.print("a53_baremetal_openocd_digilent — Digilent OpenOCD → load + break main")
  gdbforge.print("Usage: :lua a53_baremetal_openocd_digilent")
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  Digilent JTAG-HS2 (FTDI 0403:6014) connected to ZynqMP.")
  gdbforge.print("  The PS is already initialised — FSBL ran from boot.bin, or the park")
  gdbforge.print("  script below ran psu_init. This one does neither.")
  gdbforge.print("  The core is already at EL3 — it prints cpsr after halt so you can check.")
  gdbforge.print("")
  C.park_help()
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_A53_CORE=0")
  gdbforge.print("  export GDBFORGE_OPENOCD=" .. OPENOCD)
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_OPENOCD_PORT=" .. PORT)
end

function main()
  local core, bad = C.a53_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_A53_CORE must be 0|1|2|3|A0..A3 (got " .. tostring(bad) .. ")")
    return
  end

  local st = gdbforge.system("test -f " .. C.shell_quote(CFG))
  if st ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. CFG)
    return
  end

  gdbforge.print("A53 core: " .. core)
  gdbforge.print("cfg: " .. CFG)

  C.stop_openocd()
  gdbforge.print("starting openocd (Digilent HS2, A53 #" .. core .. ") …")
  gdbforge.spawn(OPENOCD, "-c", "set A53_CORE " .. core, "-f", CFG)

  if not C.wait_probe(PORT, 20, C.openocd_alive, "openocd") then
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture aarch64")
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  C.el3_note()
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = " .. C.bare_metal_entry())
  gdbforge.gdb("break main")
  gdbforge.print("a53_baremetal_openocd_digilent done — :b exec for OpenOCD logs")
end

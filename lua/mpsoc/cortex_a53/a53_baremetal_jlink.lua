-- Cortex-A53 / J-Link bare-metal bring-up.
-- Install: cp -r lua/mpsoc/cortex_a53 .gdbforge/lua/
-- Usage:   :lua a53_baremetal_jlink
--
-- Prerequisite: the core must already be at EL3, which a normally booted board is not by the
-- time you can halt it. scripts/zynqmp-park-el3.sh gets it there. It is run by hand from a
-- shell, before and outside gdbforge, with no debug session open — this script only points
-- at it. See a53_common.PARK_SCRIPT and that script's --help.
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_JLINK_CHIP     J-Link chip prefix (default XCZU3CG) → CHIP_A53_N
--   GDBFORGE_JLINK          path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE   full override e.g. XCZU3CG_A53_0 (else CHIP_A53_N)
--   GDBFORGE_JLINK_PORT     GDB listen port (default 2334)
--   GDBFORGE_A53_ENTRY      where to resume after load (default &_boot)

local C = dofile(gdbforge.lua_dir() .. "/a53_common.lua")

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local CHIP = os.getenv("GDBFORGE_JLINK_CHIP") or "XCZU3CG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"

function help()
  local core = C.a53_core() or 0
  local device = C.jlink_device(CHIP, core)
  gdbforge.print("a53_baremetal_jlink — J-Link spawn, target remote, load, break main")
  gdbforge.print("Usage: :lua a53_baremetal_jlink")
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  The PS is already initialised — FSBL ran from boot.bin, or the park")
  gdbforge.print("  script below ran psu_init. This one does neither, and unlike Xilinx")
  gdbforge.print("  XSCT it loads no FSBL and brings up no DDR or clocks of its own.")
  gdbforge.print("  It uploads your app ELF over J-Link (load + break main), nothing more.")
  gdbforge.print("  The core is already at EL3 — it prints cpsr after halt so you can check.")
  gdbforge.print("")
  C.park_help()
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_A53_CORE=0")
  gdbforge.print("  export GDBFORGE_JLINK_CHIP=" .. CHIP)
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. device)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
  gdbforge.print("After: :b exec for JLink logs")
end

function main()
  local core, bad = C.a53_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_A53_CORE must be 0|1|2|3|A0..A3 (got " .. tostring(bad) .. ")")
    return
  end
  local device = C.jlink_device(CHIP, core)
  gdbforge.print("A53 core: " .. core .. "  device: " .. device)

  C.stop_jlink()
  gdbforge.print("starting JLinkGDBServer …")
  gdbforge.spawn(
    JLINK,
    "-device", device,
    "-if", "JTAG",
    "-speed", "4000",
    "-port", PORT
  )

  if not C.wait_probe(PORT, 15, C.jlink_alive, "JLinkGDBServer") then
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
  gdbforge.print("a53_baremetal_jlink done — :b exec for JLink logs")
end

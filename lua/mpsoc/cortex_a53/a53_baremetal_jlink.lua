-- Cortex-A53 / J-Link bare-metal bring-up.
-- Install: cp -r lua/mpsoc/cortex_a53 .gdbforge/lua/
-- Usage:   :lua a53_baremetal_jlink
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_JLINK_CHIP     J-Link chip prefix (default XCZU3CG) → CHIP_A53_N
--   GDBFORGE_JLINK          path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE   full override e.g. XCZU3CG_A53_0 (else CHIP_A53_N)
--   GDBFORGE_JLINK_PORT     GDB listen port (default 2334)

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
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
  gdbforge.print("  This script uploads your app ELF over J-Link (load + break main).")
  gdbforge.print("  It does NOT load FSBL or init DDR/clocks like Xilinx XSCT does.")
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
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = 0x0")
  gdbforge.gdb("break main")
  gdbforge.print("a53_baremetal_jlink done — :b exec for JLink logs")
end

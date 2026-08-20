-- Cortex-A53 / Digilent HS2 OpenOCD bare-metal bring-up.
-- Install: cp -r lua/cortex_a53 .gdbforge/lua/
-- Usage:   :lua a53_baremetal_openocd_digilent
--
-- Env:
--   GDBFORGE_A53_CORE       APU core: 0|1|2|3|A0|A1|A2|A3 (default 0)
--   GDBFORGE_OPENOCD        path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG    OpenOCD config (default: script dir a53_openocd_digilent.cfg)
--   GDBFORGE_OPENOCD_PORT   GDB listen port (default 3333)

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
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
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
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = 0x0")
  gdbforge.gdb("break main")
  gdbforge.print("a53_baremetal_openocd_digilent done — :b exec for OpenOCD logs")
end

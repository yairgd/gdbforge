-- Cortex-R5 / J-Link bring-up.
-- Install: cp -r lua/r5_debug .gdbforge/lua/   (keeps r5_target.xml beside the script)
-- Usage:   :lua r5_debug
--
-- Env:
--   GDBFORGE_JLINK         path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE  e.g. XCZU3CG_R5_0
--   GDBFORGE_JLINK_PORT    GDB listen port (default 2334)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--
-- Uses gdbforge.spawn (background — does NOT replace the Code pane).
-- wait_port waits until JLink listens before target remote.
-- Optional: :b exec to watch JLink logs.

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local DEVICE = os.getenv("GDBFORGE_JLINK_DEVICE") or "XCZU3CG_R5_0"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"
local TDESC = os.getenv("GDBFORGE_TDESC")
  or (gdbforge.lua_dir() .. "/r5_target.xml")

function help()
  gdbforge.print("r5_baremetal_jlink — spawn JLinkGDBServer, target remote, load, break main")
  gdbforge.print("Usage: :lua r5_baremetal_jlink")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. DEVICE)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("After: :b exec for JLink logs")
end

function main()
  gdbforge.print("starting JLinkGDBServer …")
  gdbforge.spawn(
    JLINK,
    "-device", DEVICE,
    "-if", "JTAG",
    "-speed", "4000",
    "-port", PORT
  )

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 15) then
    gdbforge.print("ERROR: JLink did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("set tdesc filename " .. TDESC)
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = 0x0")
  gdbforge.gdb("break main")
  gdbforge.print("r5_debug done — Code leaf intact; :b exec for JLink logs")
end

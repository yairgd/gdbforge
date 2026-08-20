-- STM32F405 / J-Link SWD bare-metal bring-up.
-- Install: cp -r lua/stm32/stm32f405 .gdbforge/lua/
-- Usage:   :lua stm32f405_jlink
--
-- Env (edit defaults below or export before running):
--   GDBFORGE_JLINK         path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE  J-Link device name (default STM32F405RG)
--   GDBFORGE_JLINK_PORT    GDB listen port (default 2334)
--
-- Flow: kill old JLink → spawn → wait_port → target remote → reset halt → load → break main
-- Optional: :b exec for J-Link logs

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local DEVICE = os.getenv("GDBFORGE_JLINK_DEVICE") or "STM32F405RG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"

local function stop_jlink()
  gdbforge.print("stopping existing JLinkGDBServer (if any) …")
  gdbforge.system(
    "pids=$(pidof JLinkGDBServer 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

local function jlink_alive()
  local st = gdbforge.system(
    "pids=$(pidof JLinkGDBServer 2>/dev/null); " ..
    "[ -z \"$pids\" ] && exit 1; " ..
    "for p in $pids; do " ..
    "  s=$(ps -o stat= -p \"$p\" 2>/dev/null | tr -d ' '); " ..
    "  case \"$s\" in Z*) ;; *) exit 0 ;; esac; " ..
    "done; exit 1"
  )
  return st == 0
end

function help()
  gdbforge.print("stm32f405_jlink — J-Link SWD → load ELF → break main")
  gdbforge.print("Usage: :lua stm32f405_jlink")
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. DEVICE)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
  gdbforge.print("")
  gdbforge.print("Edit DEVICE at the top of this file for other STM32F4 parts (F407, F411, …).")
end

function main()
  gdbforge.print("device: " .. DEVICE .. "  port: " .. PORT)
  stop_jlink()

  gdbforge.print("starting JLinkGDBServer (SWD) …")
  gdbforge.spawn(
    JLINK,
    "-device", DEVICE,
    "-if", "SWD",
    "-speed", "4000",
    "-port", PORT
  )

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 15) then
    gdbforge.print("ERROR: JLink did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.sleep(0.5)
  if not jlink_alive() then
    gdbforge.print("ERROR: JLinkGDBServer died — check probe/USB, :b exec")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor reset halt")
  gdbforge.gdb("load")
  gdbforge.gdb("break main")
  gdbforge.print("stm32f405_jlink done — :b exec for J-Link logs")
end

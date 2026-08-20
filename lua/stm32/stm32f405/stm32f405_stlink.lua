-- STM32F405 / ST-Link (OpenOCD) bare-metal bring-up.
-- Install: cp -r lua/stm32/stm32f405 .gdbforge/lua/   (keeps stm32f405_openocd.cfg)
-- Usage:   :lua stm32f405_stlink
--
-- Env:
--   GDBFORGE_OPENOCD       path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG   config file (default: script dir stm32f405_openocd.cfg)
--   GDBFORGE_OPENOCD_PORT  GDB listen port (default 3333)
--
-- Flow: kill old openocd → spawn → wait_port → target remote → reset halt → load → break main
-- Optional: :b exec for OpenOCD logs

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/stm32f405_openocd.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

local function stop_openocd()
  gdbforge.print("stopping existing openocd (if any) …")
  gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

local function openocd_alive()
  local st = gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "[ -z \"$pids\" ] && exit 1; " ..
    "for p in $pids; do " ..
    "  s=$(ps -o stat= -p \"$p\" 2>/dev/null | tr -d ' '); " ..
    "  case \"$s\" in Z*) ;; *) exit 0 ;; esac; " ..
    "done; exit 1"
  )
  return st == 0
end

function help()
  gdbforge.print("stm32f405_stlink — ST-Link + OpenOCD → load ELF → break main")
  gdbforge.print("Usage: :lua stm32f405_stlink")
  gdbforge.print("")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_OPENOCD=" .. OPENOCD)
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_OPENOCD_PORT=" .. PORT)
  gdbforge.print("")
  gdbforge.print("Edit stm32f405_openocd.cfg for adapter speed or a different F4 target.")
end

function main()
  local st = gdbforge.system("test -f " .. shell_quote(CFG))
  if st ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. CFG)
    return
  end
  gdbforge.print("cfg: " .. CFG .. "  port: " .. PORT)
  stop_openocd()

  gdbforge.print("starting openocd (ST-Link SWD) …")
  gdbforge.spawn(OPENOCD, "-f", CFG)

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 20) then
    gdbforge.print("ERROR: OpenOCD did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.sleep(0.5)
  if not openocd_alive() then
    gdbforge.print("ERROR: openocd died — check ST-Link/USB, :b exec")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor reset halt")
  gdbforge.gdb("load")
  gdbforge.gdb("break main")
  gdbforge.print("stm32f405_stlink done — :b exec for OpenOCD logs")
end

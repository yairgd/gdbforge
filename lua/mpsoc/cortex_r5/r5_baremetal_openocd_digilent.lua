-- Cortex-R5 / Digilent HS2 OpenOCD bring-up.
-- Install: copy lua/mpsoc/cortex_r5 into .gdbforge/lua/ (keeps cfg + r5_target.xml)
-- Usage:   :lua r5_baremetal_openocd_digilent
--
-- Env:
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--   GDBFORGE_OPENOCD       path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG   OpenOCD config (default: script dir r5_openocd_digilent.cfg)
--   GDBFORGE_OPENOCD_PORT  GDB listen port (default 3333)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--
-- Kills any existing openocd, then gdbforge.spawn (background — Code pane stays).
-- wait_port waits until OpenOCD listens before target remote.
-- Optional: :b exec to watch OpenOCD logs.

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/r5_openocd_digilent.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"
local TDESC = os.getenv("GDBFORGE_TDESC")
  or (gdbforge.lua_dir() .. "/r5_target.xml")

-- Parse GDBFORGE_R5_CORE → 0 or 1 (default 0). Accepts 0|1|R0|R1.
local function r5_core()
  local v = os.getenv("GDBFORGE_R5_CORE") or "0"
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):upper():gsub("^R", "")
  if v == "0" or v == "1" then
    return tonumber(v)
  end
  return nil, v
end

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

-- Stop a leftover OpenOCD so the GDB port is free before respawn.
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

-- True if a non-zombie openocd is still running.
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
  gdbforge.print("r5_baremetal_openocd_digilent — Digilent HS2 OpenOCD → load + break main")
  gdbforge.print("Usage: :lua r5_baremetal_openocd_digilent")
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  Digilent JTAG-HS2 (FTDI 0403:6014) connected to ZynqMP.")
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
  gdbforge.print("  This script only uploads your app ELF over OpenOCD (load + break main).")
  gdbforge.print("  It does NOT load FSBL or init DDR/clocks like Xilinx XSCT does.")
  gdbforge.print("")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_OPENOCD=" .. OPENOCD)
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_OPENOCD_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("After: :b exec for OpenOCD logs")
end

function main()
  local core, bad = r5_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_R5_CORE must be 0|1|R0|R1 (got " .. tostring(bad) .. ")")
    return
  end

  local st = gdbforge.system("test -f " .. shell_quote(CFG))
  if st ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. CFG)
    return
  end
  st = gdbforge.system("test -f " .. shell_quote(TDESC))
  if st ~= 0 then
    gdbforge.print("ERROR: tdesc not found: " .. TDESC)
    return
  end
  gdbforge.print("R5 core: R" .. core)
  gdbforge.print("cfg: " .. CFG)
  gdbforge.print("tdesc: " .. TDESC)

  stop_openocd()

  gdbforge.print("starting openocd (Digilent HS2, R" .. core .. ") …")
  -- -c before -f so the cfg can read R5_CORE when selecting the GDB target.
  gdbforge.spawn(OPENOCD, "-c", "set R5_CORE " .. core, "-f", CFG)

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 20) then
    gdbforge.print("ERROR: OpenOCD did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")
  gdbforge.sleep(0.5)
  if not openocd_alive() then
    gdbforge.print("ERROR: openocd died after bind (zombie/crash) — check probe/USB, :b exec")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("set tdesc filename " .. TDESC)
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = 0x0")
  gdbforge.gdb("break main")
  gdbforge.print("r5_baremetal_openocd_digilent done — Code leaf intact; :b exec for OpenOCD logs")
end

-- Cortex-R5 / J-Link bring-up.
-- Install: copy lua/cortex_r5 into .gdbforge/lua/ (keeps r5_target.xml beside the script)
-- Usage:   :lua r5_baremetal_jlink
--
-- Env:
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--   GDBFORGE_JLINK_CHIP    J-Link chip prefix (default XCZU3CG) → CHIP_R5_N
--   GDBFORGE_JLINK         path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE  full override e.g. XCZU3CG_R5_0 (else CHIP_R5_N;
--                          trailing _R5_N rewritten from R5_CORE when set)
--   GDBFORGE_JLINK_PORT    GDB listen port (default 2334)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--
-- Kills any existing JLinkGDBServer, then gdbforge.spawn (background — Code pane stays).
-- wait_port waits until JLink listens before target remote.
-- Optional: :b exec to watch JLink logs.

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local CHIP = os.getenv("GDBFORGE_JLINK_CHIP") or "XCZU3CG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"
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

local function jlink_device(core)
  local d = os.getenv("GDBFORGE_JLINK_DEVICE")
  if d == nil or d == "" then
    return CHIP .. "_R5_" .. core
  end
  if d:match("_R5_%d+$") then
    return (d:gsub("_R5_%d+$", "_R5_" .. core))
  end
  return d
end

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

-- Stop a leftover J-Link so -port is free before respawn (pidof + kill; no pkill).
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

-- True if a non-zombie JLinkGDBServer is still running.
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
  local core = r5_core() or 0
  local device = jlink_device(core)
  gdbforge.print("r5_baremetal_jlink — kill old JLink, spawn, target remote, load, break main")
  gdbforge.print("Usage: :lua r5_baremetal_jlink")
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
  gdbforge.print("  This script only uploads your app ELF over J-Link (load + break main).")
  gdbforge.print("  It does NOT load FSBL or init DDR/clocks like Xilinx XSCT does.")
  gdbforge.print("  Full FSBL-then-app bring-up needs extra steps — not in this script.")
  gdbforge.print("")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_JLINK_CHIP=" .. CHIP)
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. device)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("After: :b exec for JLink logs")
end

function main()
  local core, bad = r5_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_R5_CORE must be 0|1|R0|R1 (got " .. tostring(bad) .. ")")
    return
  end
  local device = jlink_device(core)

  -- tdesc is self-contained (no separate register.xml). Fail early if missing.
  local st = gdbforge.system("test -f " .. shell_quote(TDESC))
  if st ~= 0 then
    gdbforge.print("ERROR: tdesc not found: " .. TDESC)
    return
  end
  gdbforge.print("R5 core: R" .. core .. "  device: " .. device)
  gdbforge.print("tdesc: " .. TDESC)

  stop_jlink()

  gdbforge.print("starting JLinkGDBServer …")
  gdbforge.spawn(
    JLINK,
    "-device", device,
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
  gdbforge.sleep(0.5)
  if not jlink_alive() then
    gdbforge.print("ERROR: JLinkGDBServer died after bind (zombie/crash) — check probe/USB, :b exec")
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
  gdbforge.print("r5_baremetal_jlink done — Code leaf intact; :b exec for JLink logs")
end

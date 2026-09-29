-- Shared helpers for cortex_a53 Lua scripts (J-Link + Digilent OpenOCD).

local M = {}

-- Companion xsdb script in the gdbforge tree that puts a ZynqMP board into the state the
-- bare-metal scripts here need: JTAG boot mode so no FSBL/ATF/U-Boot runs, psu_init for the
-- clocks and MIO FSBL would have configured, and an A53 left halted at EL3.
--
-- Nothing here runs it, and nothing here can: it needs hw_server to own the JTAG cable
-- exclusively, and gdbforge is holding that cable through openocd or JLinkGDBServer for as
-- long as a session is open. The user runs it from a shell, with no session open, and only
-- then starts one of these scripts. The path is printed so it can be found; "--help" on it
-- explains the whole story.
M.PARK_SCRIPT = "scripts/zynqmp-park-el3.sh"

function M.trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

function M.env(name, fallback)
  local v = os.getenv(name)
  if v == nil or M.trim(v) == "" then
    return fallback
  end
  return M.trim(v)
end

-- Parse GDBFORGE_A53_CORE → 0..3 (default 0). Accepts 0|1|2|3|A0|A1|A2|A3.
function M.a53_core()
  local v = os.getenv("GDBFORGE_A53_CORE") or "0"
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):upper():gsub("^A", "")
  local n = tonumber(v)
  if n and n >= 0 and n <= 3 then
    return n
  end
  return nil, v
end

function M.jlink_device(chip, core)
  local d = os.getenv("GDBFORGE_JLINK_DEVICE")
  if d == nil or d == "" then
    return chip .. "_A53_" .. core
  end
  if d:match("_A53_%d+$") then
    return (d:gsub("_A53_%d+$", "_A53_" .. core))
  end
  return d
end

function M.stop_jlink()
  gdbforge.print("stopping existing JLinkGDBServer (if any) …")
  gdbforge.system(
    "pids=$(pidof JLinkGDBServer 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

function M.jlink_alive()
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

function M.stop_openocd()
  gdbforge.print("stopping existing openocd (if any) …")
  gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

function M.openocd_alive()
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

function M.wait_probe(port, timeout, alive_fn, probe_name)
  gdbforge.print("waiting for port " .. port .. " …")
  if not gdbforge.wait_port(port, timeout) then
    gdbforge.print("ERROR: " .. probe_name .. " did not listen on :" .. port .. " — try :b exec")
    return false
  end
  gdbforge.print("port " .. port .. " is open")
  gdbforge.sleep(0.5)
  if not alive_fn() then
    gdbforge.print("ERROR: " .. probe_name .. " died after bind — check probe/USB, :b exec")
    return false
  end
  return true
end

local function kgdb_common_candidates()
  local rel = "/kgdb_common/kgdb_common.lua"
  local list = { gdbforge.lua_dir() .. "/.." .. rel, "./.gdbforge/lua" .. rel }
  local home = os.getenv("HOME")
  if home and home ~= "" then
    list[#list + 1] = home .. "/.gdbforge/lua" .. rel
    list[#list + 1] = home .. "/.cache/gdbforge/embedded-lua" .. rel
    list[#list + 1] = home .. "/.cache/gdbforge/embedded-lua/kernel" .. rel
  end
  return list
end

function M.load_kgdb_common()
  for _, path in ipairs(kgdb_common_candidates()) do
    local fh = io.open(path, "r")
    if fh then
      fh:close()
      local ok, err = pcall(function() dofile(path) end)
      if ok then return kgdb_common end
      gdbforge.print("ERROR: cannot load kgdb_common: " .. tostring(err))
      return nil
    end
  end
  gdbforge.print("ERROR: kgdb_common.lua not found — cp -r lua/kernel/kgdb_common .gdbforge/lua/")
  return nil
end

function M.resolve_vmlinux(arg)
  local v = M.trim(arg or "")
  if v == "" then
    v = M.env("GDBFORGE_KGDB_VMLINUX", "")
  end
  if v == "" then
    local C = M.load_kgdb_common()
    if C then
      v = C.kgdb_vmlinux()
    end
  end
  return v
end

-- Where to resume a freshly loaded bare-metal app. "_boot" is the Xilinx standalone BSP
-- reset label, so it resolves to wherever the app was actually linked: DDR 0x0 for a default
-- Vitis lscript.ld, OCM 0xFFFC0000 for an FSBL-style one. A hardcoded address only ever suits
-- one of those, which is why this is a symbol.
function M.bare_metal_entry()
  return M.env("GDBFORGE_A53_ENTRY", "&_boot")
end

-- The standalone BSP is built EL3=1 / EL1_NONSECURE=0, so boot.S has only an EL3 entry path:
-- it reads currentEL, and anything other than EL3 falls through to "b error" and spins there
-- before _startup or main ever run. A core cannot raise its own exception level, so the app
-- has to start on a core that is still at EL3.
--
-- That rules out the obvious halt point. On a board booting FSBL -> ATF -> U-Boot, only FSBL
-- and ATF run at EL3; ATF hands U-Boot down to EL2, so by the time there is a U-Boot prompt
-- to interrupt, EL3 is already gone.
--
-- gdbforge.gdb() returns nothing to Lua, so this cannot branch on the result — it puts the
-- value in front of the user instead, which is the difference between a one-line diagnosis
-- and a long session staring at a spin loop.
function M.el3_note()
  gdbforge.gdb("p/x $cpsr")
  gdbforge.print("cpsr is in the gdb buffer: mode nibble D = EL3, 9 = EL2, 5 = EL1.")
  gdbforge.print("Only EL3 will run — boot.S spins at \"error\" for EL2/EL1, no UART output.")
  gdbforge.print("Not D? Halting later cannot fix it. Close this session and park the core:")
  gdbforge.print("  " .. M.PARK_SCRIPT .. " -p <platform>/hw/psu_init.tcl")
  gdbforge.print("then rerun this script. Do not let the debugger reset the target afterwards.")
end

-- Printed by help() in the bare-metal scripts. A core at EL2 is the single most common way
-- this whole flow fails, and the fix is a script the user has to run outside gdbforge, so
-- help() has to explain both. Kept next to el3_note so the two stay in step.
function M.park_help()
  gdbforge.print("EL2 instead of EL3 — why this matters, and the script that fixes it:")
  gdbforge.print("  An app built EL3=1 / EL1_NONSECURE=0 has one entry path and it is the EL3")
  gdbforge.print("  one. boot.S reads currentEL and sends anything else to \"b error\", a two-")
  gdbforge.print("  instruction spin reached before _startup and main: no UART output at all,")
  gdbforge.print("  which reads as a bad ELF or a broken probe. gdb cannot undo it — a core")
  gdbforge.print("  cannot raise its own exception level, and load / set $pc run at whatever")
  gdbforge.print("  EL it is already at. A board that booted normally is past EL3 by the time")
  gdbforge.print("  you can halt it: ATF hands U-Boot down to EL2, so the U-Boot prompt is")
  gdbforge.print("  EL2 (cpsr mode nibble 9). The answer is not to boot the board at all.")
  gdbforge.print("")
  gdbforge.print("  Run this first, in a shell, with no gdbforge session open:")
  gdbforge.print("      " .. M.PARK_SCRIPT .. " -p <platform>/hw/psu_init.tcl")
  gdbforge.print("  It resets into JTAG boot mode so no FSBL/ATF/U-Boot runs, runs psu_init")
  gdbforge.print("  for the clocks and MIO FSBL would have set, and leaves the A53 halted at")
  gdbforge.print("  EL3. gdbforge cannot do this for you: the script needs the JTAG cable to")
  gdbforge.print("  itself, and a live session is holding it. Then rerun this script, and do")
  gdbforge.print("  not let the debugger reset the target — a reset discards psu_init.")
  gdbforge.print("  The board stays in JTAG boot mode afterwards: --clear-boot-mode undoes it.")
end

function M.kernel_prereq_help()
  gdbforge.print("Kernel JTAG prerequisites:")
  gdbforge.print("  Linux running on A53; matching vmlinux on host (GDBFORGE_KGDB_VMLINUX)")
  gdbforge.print("  Disable cpuidle on the debug core or JTAG may drop when the core idles")
  gdbforge.print("  For day-to-day kernel debug, UART kgdb (kgdb_kdmx) is usually easier")
end

return M

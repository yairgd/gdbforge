-- Shared helpers for cortex_a53 Lua scripts (J-Link + Digilent OpenOCD).

local M = {}

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

function M.kernel_prereq_help()
  gdbforge.print("Kernel JTAG prerequisites:")
  gdbforge.print("  Linux running on A53; matching vmlinux on host (GDBFORGE_KGDB_VMLINUX)")
  gdbforge.print("  Disable cpuidle on the debug core or JTAG may drop when the core idles")
  gdbforge.print("  For day-to-day kernel debug, UART kgdb (kgdb_kdmx) is usually easier")
end

return M

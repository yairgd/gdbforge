-- Cortex-R5 OpenAMP bring-up (Linux remoteproc + Digilent HS2 OpenOCD attach).
-- Install: cp -r lua/cortex_r5 .gdbforge/lua/   (keeps cfg + r5_target.xml)
-- Usage:
--   :lua r5_openamp_openocd_digilent
--   :lua r5_openamp_openocd_digilent ./firmware
--
-- On the target (A53 / Linux):
--   1) scp R5 firmware → /lib/firmware/
--   2) stop/start remoteprocN (N = GDBFORGE_R5_CORE) with that firmware
-- Then on the host: OpenOCD (Digilent HS2) →
-- GDB target remote (no load — remoteproc already loaded the image).
--
-- Env (core + OpenOCD — same defaults as r5_baremetal_openocd_digilent):
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--                          → remoteprocN + OpenOCD R5_CORE
--   GDBFORGE_OPENOCD       path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG   OpenOCD config (default: script dir r5_openocd_digilent.cfg)
--   GDBFORGE_OPENOCD_PORT  GDB listen port (default 3333)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--
-- Env (board / firmware — same as r5_openamp_jlink):
--   GDBFORGE_REMOTE_HOST   (default 192.168.20.50)
--   GDBFORGE_REMOTE_USER   (default root)
--   GDBFORGE_R5_FW         local R5 firmware path (or pass as :lua arg / gdbforge.program())
--   GDBFORGE_R5_FW_NAME    remoteproc firmware name under /lib/firmware
--                          (default: basename of the firmware file)

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/r5_openocd_digilent.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"
local TDESC = os.getenv("GDBFORGE_TDESC")
  or (gdbforge.lua_dir() .. "/r5_target.xml")

local DEFAULT_HOST = "192.168.20.50"
local DEFAULT_USER = "root"
local REMOTE_FW_DIR = "/lib/firmware"

-- Parse GDBFORGE_R5_CORE → 0 or 1 (default 0). Accepts 0|1|R0|R1.
local function r5_core()
  local v = os.getenv("GDBFORGE_R5_CORE") or "0"
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):upper():gsub("^R", "")
  if v == "0" or v == "1" then
    return tonumber(v)
  end
  return nil, v
end

function help()
  gdbforge.print("r5_openamp_openocd_digilent — remoteproc + Digilent OpenOCD attach (no load)")
  gdbforge.print("Usage: :lua r5_openamp_openocd_digilent [firmware]")
  gdbforge.print("  :lua r5_openamp_openocd_digilent ./firmware")
  gdbforge.print("Attach: openocd → target remote → halt → break main (no load)")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_OPENOCD=" .. OPENOCD)
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_OPENOCD_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("  export GDBFORGE_REMOTE_HOST=" .. DEFAULT_HOST)
  gdbforge.print("  export GDBFORGE_REMOTE_USER=" .. DEFAULT_USER)
  gdbforge.print("  export GDBFORGE_R5_FW=./firmware")
  gdbforge.print("  export GDBFORGE_R5_FW_NAME=firmware")
end

local function trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function env(name, fallback)
  local v = os.getenv(name)
  if v == nil or trim(v) == "" then
    return fallback
  end
  return trim(v)
end

local function basename(path)
  path = tostring(path or ""):gsub("\\", "/")
  local i = path:match("^.*()/")
  if i then
    return path:sub(i + 1)
  end
  return path
end

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

local function run_cmd(cmd)
  local code, out = gdbforge.system(cmd)
  return tonumber(code) or 1, out or ""
end

local function scp_to(local_path, user, host, remote_path)
  local scp = string.format(
    "scp -o BatchMode=yes -o ConnectTimeout=15 %s %s@%s:%s",
    shell_quote(local_path), user, host, shell_quote(remote_path)
  )
  local code, out = run_cmd(scp)
  if code ~= 0 then
    gdbforge.print("ERROR: scp failed (" .. local_path .. " → " .. remote_path .. "):")
    gdbforge.print(trim(out))
    return false
  end
  gdbforge.print("copied → " .. user .. "@" .. host .. ":" .. remote_path)
  return true
end

local function remote_sh(user, host, script)
  local cmd = string.format(
    "ssh -o BatchMode=yes -o ConnectTimeout=15 %s@%s %s",
    user, host, shell_quote(script)
  )
  return run_cmd(cmd)
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

function main(fw_arg)
  local core, bad = r5_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_R5_CORE must be 0|1|R0|R1 (got " .. tostring(bad) .. ")")
    return
  end
  local rproc = "remoteproc" .. core

  local host = env("GDBFORGE_REMOTE_HOST", DEFAULT_HOST)
  local user = env("GDBFORGE_REMOTE_USER", DEFAULT_USER)

  local fw = trim(fw_arg)
  if fw == "" then
    fw = env("GDBFORGE_R5_FW", "")
  end
  if fw == "" then
    fw = trim(gdbforge.program() or "")
  end
  if fw == "" then
    gdbforge.print("ERROR: set R5 firmware — :lua r5_openamp_openocd_digilent ./firmware")
    gdbforge.print("       or GDBFORGE_R5_FW / start gdbforge with the ELF")
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

  local fw_name = env("GDBFORGE_R5_FW_NAME", basename(fw))
  local remote_fw = REMOTE_FW_DIR .. "/" .. fw_name

  gdbforge.print("r5_openamp_openocd_digilent: " .. fw .. " → " .. user .. "@" .. host)
  gdbforge.print("R5 core: R" .. core .. "  " .. rproc)
  gdbforge.print("remoteproc firmware name: " .. fw_name)
  gdbforge.print("cfg: " .. CFG)

  -- 1) copy R5 image into /lib/firmware on the target
  if not scp_to(fw, user, host, remote_fw) then
    return
  end

  -- 2) remoteproc stop / set firmware / start
  local rproc_dir = "/sys/class/remoteproc/" .. rproc
  local bringup = table.concat({
    "echo stop > " .. rproc_dir .. "/state 2>/dev/null || true",
    "echo " .. shell_quote(fw_name) .. " > " .. rproc_dir .. "/firmware",
    "echo start > " .. rproc_dir .. "/state",
  }, "; ")
  gdbforge.print(rproc .. " bring-up on target …")
  local code, out = remote_sh(user, host, bringup)
  if code ~= 0 then
    gdbforge.print("ERROR: remoteproc bring-up failed:")
    gdbforge.print(trim(out))
    return
  end
  if trim(out) ~= "" then
    gdbforge.print(trim(out))
  end
  gdbforge.print(rproc .. " started")

  gdbforge.sleep(1)

  -- 3) OpenOCD attach (no load — unlike baremetal)
  stop_openocd()
  gdbforge.print("starting openocd (Digilent HS2 attach, R" .. core .. ") …")
  gdbforge.spawn(OPENOCD, "-c", "set R5_CORE " .. core, "-f", CFG)

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 20) then
    gdbforge.print("ERROR: OpenOCD did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")
  gdbforge.sleep(0.5)
  if not openocd_alive() then
    gdbforge.print("ERROR: openocd died after bind — check probe/USB, :b exec")
    return
  end

  gdbforge.open_buffer("gdb")
  -- Symbols from the host ELF (firmware already loaded by remoteproc).
  gdbforge.gdb("file " .. fw)
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("set tdesc filename " .. TDESC)
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  gdbforge.gdb("break main")
  gdbforge.print("r5_openamp_openocd_digilent done — attached (no load); :b exec for OpenOCD logs")
end

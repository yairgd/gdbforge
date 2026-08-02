-- Cortex-R5 OpenAMP bring-up (Linux remoteproc + J-Link attach).
-- Install: cp -r lua/cortex_r5 .gdbforge/lua/   (keeps r5_target.xml beside the script)
-- Usage:
--   :lua r5_openamp_jlink
--   :lua r5_openamp_jlink ./firmware
--
-- On the target (A53 / Linux):
--   1) scp R5 firmware → /lib/firmware/
--   2) stop/start remoteprocN (N = GDBFORGE_R5_CORE) with that firmware
-- Then on the host: JLinkGDBServer in attach mode (-noreset -noir) →
-- GDB target remote (no load — remoteproc already loaded the image).
--
-- Env (core + J-Link — same defaults as r5_baremetal_jlink):
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--                          → remoteprocN + J-Link device …_R5_N
--   GDBFORGE_JLINK_CHIP    J-Link chip prefix (default XCZU3CG) → CHIP_R5_N
--   GDBFORGE_JLINK         path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE  full override e.g. XCZU3CG_R5_0 (else CHIP_R5_N;
--                          trailing _R5_N rewritten from R5_CORE when set)
--   GDBFORGE_JLINK_PORT    GDB listen port (default 2334)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--
-- Env (board / firmware):
--   GDBFORGE_REMOTE_HOST   (default 192.168.20.50)
--   GDBFORGE_REMOTE_USER   (default root)
--   GDBFORGE_R5_FW         local R5 firmware path (or pass as :lua arg / gdbforge.program())
--   GDBFORGE_R5_FW_NAME    remoteproc firmware name under /lib/firmware
--                          (default: basename of the firmware file)

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local CHIP = os.getenv("GDBFORGE_JLINK_CHIP") or "XCZU3CG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"
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

function help()
  local core = r5_core() or 0
  local device = jlink_device(core)
  gdbforge.print("r5_openamp_jlink — remoteproc bring-up + J-Link attach (no reset/load)")
  gdbforge.print("Usage: :lua r5_openamp_jlink [firmware]")
  gdbforge.print("  :lua r5_openamp_jlink ./firmware")
  gdbforge.print("Attach: JLinkGDBServer -noreset -noir → target remote → halt → break main")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_JLINK_CHIP=" .. CHIP)
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. device)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
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

-- Prefer cancellable host API (Ctrl-C kills process group during ssh/scp).
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

function main(fw_arg)
  local core, bad = r5_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_R5_CORE must be 0|1|R0|R1 (got " .. tostring(bad) .. ")")
    return
  end
  local device = jlink_device(core)
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
    gdbforge.print("ERROR: set R5 firmware — :lua r5_openamp_jlink ./firmware")
    gdbforge.print("       or GDBFORGE_R5_FW / start gdbforge with the ELF")
    return
  end

  local fw_name = env("GDBFORGE_R5_FW_NAME", basename(fw))
  local remote_fw = REMOTE_FW_DIR .. "/" .. fw_name

  gdbforge.print("r5_openamp_jlink: " .. fw .. " → " .. user .. "@" .. host)
  gdbforge.print("R5 core: R" .. core .. "  " .. rproc .. "  device: " .. device)
  gdbforge.print("remoteproc firmware name: " .. fw_name)

  -- 1) copy R5 image into /lib/firmware on the target
  if not scp_to(fw, user, host, remote_fw) then
    return
  end

  -- 2) remoteproc stop / set firmware / start (remoteprocN from GDBFORGE_R5_CORE)
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

  -- brief settle before J-Link attach
  gdbforge.sleep(1)

  -- 3) J-Link attach mode (no reset) + target remote — unlike baremetal (load).
  gdbforge.print("starting JLinkGDBServer (attach: -noreset -noir) …")
  gdbforge.spawn(
    JLINK,
    "-device", device,
    "-if", "JTAG",
    "-speed", "4000",
    "-port", PORT,
    "-noreset",
    "-noir"
  )

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 15) then
    gdbforge.print("ERROR: JLink did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")

  gdbforge.open_buffer("gdb")
  -- Symbols from the host ELF (firmware already loaded by remoteproc).
  gdbforge.gdb("file " .. fw)
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("set tdesc filename " .. TDESC)
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  gdbforge.gdb("break main")
  gdbforge.print("r5_openamp_jlink done — attached (no load); :b exec for JLink logs")
end

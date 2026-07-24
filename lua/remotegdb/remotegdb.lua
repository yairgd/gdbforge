-- remotegdb — debug a binary on an embedded Linux board via scp + gdbserver + GDB.
--
-- Install:
--   mkdir -p .gdbforge/lua
--   cp -r /path/to/gdbforge/lua/remotegdb .gdbforge/lua/
--
-- Typical session (GDB backend — not Delve):
--   ./bin/gdbforge ./hello          # or rely on GDBFORGE_REMOTE_APP / :lua arg
--   :lua remotegdb
--   :lua remotegdb ./hello
--   :lua remotegdb ./hello 192.168.20.50 1234
--
-- Flow:
--   1) Resolve local app (arg → GDBFORGE_REMOTE_APP → gdbforge.program())
--   2) Compare local vs remote MD5; scp to /tmp only if missing or changed
--   3) Open a terminal: ssh -t user@host 'gdbserver :PORT /tmp/<app>'
--   4) wait_port on the board, then: file <app> ; target remote host:PORT
--
-- Env (all optional — edit defaults below or export before gdbforge):
--   GDBFORGE_REMOTE_APP    local binary path (placeholder if unset)
--   GDBFORGE_REMOTE_HOST   board IP/hostname          (default 192.168.20.50)
--   GDBFORGE_REMOTE_USER   ssh user                   (default root)
--   GDBFORGE_REMOTE_PORT   gdbserver listen port      (default 1234)
--   GDBFORGE_REMOTE_DIR    remote directory           (default /tmp)
--   GDBFORGE_TERMINAL      mate-terminal|kitty|xterm|… (external ssh window)
--
-- Requires: ssh, scp, md5sum (or md5) on the host; gdbserver on the board PATH.

-- Placeholder: set your board defaults here, or override with env / :lua args.
local DEFAULT_HOST = "192.168.20.50"
local DEFAULT_USER = "root"
local DEFAULT_PORT = "1234"
local DEFAULT_DIR = "/tmp"
-- Leave empty to force GDBFORGE_REMOTE_APP / session program / :lua arg.
local DEFAULT_APP = ""

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

-- Run a shell command; return exit code (0 = ok) and combined stdout+stderr text.
local function run_cmd(cmd)
  local f = io.popen(cmd .. " 2>&1", "r")
  if not f then
    return 1, "io.popen failed"
  end
  local out = f:read("*a") or ""
  local ok, _, code = f:close()
  if ok == true or code == 0 then
    return 0, out
  end
  if type(code) == "number" then
    return code, out
  end
  return 1, out
end

local function local_md5(path)
  local code, out = run_cmd("md5sum " .. shell_quote(path))
  if code ~= 0 then
    code, out = run_cmd("md5 -q " .. shell_quote(path))
  end
  if code ~= 0 then
    return nil, out
  end
  local hash = out:match("([0-9a-fA-F]+)")
  return hash, out
end

local function remote_md5(user, host, remote_path)
  local remote = string.format(
    "ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s md5sum %s 2>/dev/null || ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s md5 -q %s 2>/dev/null",
    user, host, shell_quote(remote_path),
    user, host, shell_quote(remote_path)
  )
  local code, out = run_cmd(remote)
  if code ~= 0 then
    return nil, out
  end
  local hash = out:match("([0-9a-fA-F]+)")
  return hash, out
end

local function ensure_deployed(local_app, user, host, remote_path)
  local lhash, lerr = local_md5(local_app)
  if not lhash then
    gdbforge.print("ERROR: cannot hash local app: " .. tostring(lerr))
    return false
  end
  gdbforge.print("local md5:  " .. lhash)

  local rhash = remote_md5(user, host, remote_path)
  if rhash and rhash:lower() == lhash:lower() then
    gdbforge.print("remote already up to date — skip scp (" .. remote_path .. ")")
    return true
  end
  if rhash then
    gdbforge.print("remote md5: " .. rhash .. " — copying …")
  else
    gdbforge.print("remote missing or unreachable hash — copying …")
  end

  local scp = string.format(
    "scp -o BatchMode=yes -o ConnectTimeout=15 %s %s@%s:%s",
    shell_quote(local_app), user, host, shell_quote(remote_path)
  )
  local code, out = run_cmd(scp)
  if code ~= 0 then
    gdbforge.print("ERROR: scp failed:")
    gdbforge.print(trim(out))
    return false
  end
  -- Ensure executable bit on target.
  run_cmd(string.format(
    "ssh -o BatchMode=yes %s@%s chmod +x %s",
    user, host, shell_quote(remote_path)
  ))
  gdbforge.print("copied → " .. user .. "@" .. host .. ":" .. remote_path)
  return true
end

function main(app, host, port)
  app = trim(app)
  if app == "" then
    app = env("GDBFORGE_REMOTE_APP", DEFAULT_APP)
  end
  if app == "" then
    app = trim(gdbforge.program() or "")
  end
  if app == "" then
    gdbforge.print("ERROR: set app path — :lua remotegdb ./myapp")
    gdbforge.print("  or export GDBFORGE_REMOTE_APP=./myapp")
    gdbforge.print("  or start: gdbforge ./myapp")
    gdbforge.print("  or edit DEFAULT_APP in remotegdb.lua")
    return
  end

  host = trim(host)
  if host == "" then
    host = env("GDBFORGE_REMOTE_HOST", DEFAULT_HOST)
  end
  port = trim(port)
  if port == "" then
    port = env("GDBFORGE_REMOTE_PORT", DEFAULT_PORT)
  end
  local user = env("GDBFORGE_REMOTE_USER", DEFAULT_USER)
  local rdir = env("GDBFORGE_REMOTE_DIR", DEFAULT_DIR):gsub("/+$", "")
  local remote_path = rdir .. "/" .. basename(app)
  local addr = host .. ":" .. port

  gdbforge.print("remotegdb: " .. app .. " → " .. user .. "@" .. host .. ":" .. remote_path)
  gdbforge.print("gdbserver port " .. port .. " (terminal: GDBFORGE_TERMINAL=" ..
    (os.getenv("GDBFORGE_TERMINAL") or "auto") .. ")")

  if not ensure_deployed(app, user, host, remote_path) then
    return
  end

  local remote_cmd = string.format("gdbserver :%s %s", port, remote_path)
  gdbforge.print("opening terminal: ssh -t " .. user .. "@" .. host .. " " .. remote_cmd)
  gdbforge.spawn_terminal("ssh", "-t", user .. "@" .. host, remote_cmd)

  gdbforge.print("waiting for " .. addr .. " …")
  if not gdbforge.wait_port(addr, 30) then
    gdbforge.print("ERROR: nothing listening on " .. addr)
    gdbforge.print("  check the ssh/gdbserver window, board IP, firewall, gdbserver PATH")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("file " .. app)
  gdbforge.gdb("target remote " .. addr)
  gdbforge.print("attached — symbols from " .. app .. "; remote " .. addr)
  gdbforge.print("next: break main ; continue   (or set BPs then c)")
end

-- remotegdb — scp (if needed) + start gdbserver on board + GDB target remote.
--
-- Usage (GDB backend):
--   :lua remotegdb
--   :lua remotegdb ./hello
--   :lua remotegdb ./hello 192.168.20.50 1234
--
-- Env (optional):
--   GDBFORGE_REMOTE_APP GDBFORGE_REMOTE_HOST GDBFORGE_REMOTE_USER
--   GDBFORGE_REMOTE_PORT GDBFORGE_REMOTE_DIR

local DEFAULT_HOST = "192.168.20.50"
local DEFAULT_USER = "root"
local DEFAULT_PORT = "1234"
local DEFAULT_DIR = "/tmp"
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

-- gopher-lua: popen close() returns exit status number (0 = ok), not true.
local function run_cmd(cmd)
  local f = io.popen(cmd .. " 2>&1", "r")
  if not f then
    return 1, "io.popen failed"
  end
  local out = f:read("*a") or ""
  local status = f:close()
  if status == true or status == 0 then
    return 0, out
  end
  if type(status) == "number" then
    return status, out
  end
  return 1, out
end

local function local_md5(path)
  local code, out = run_cmd("/usr/bin/md5sum " .. shell_quote(path))
  local hash = tostring(out or ""):match("([0-9a-fA-F]+)")
  if hash then
    return hash, out
  end
  return nil, out ~= "" and out or ("md5 failed, status=" .. tostring(code))
end

local function remote_md5(user, host, remote_path)
  local remote = string.format(
    "ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s /usr/bin/md5sum %s 2>/dev/null || ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s md5 -q %s 2>/dev/null",
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

  -- 1) copy (md5)
  if not ensure_deployed(app, user, host, remote_path) then
    return
  end

  -- 2) open gdbserver on the target (no host spawn / no external terminal)
local start = string.format(
  "ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s %s",
  user, host,
  shell_quote(string.format(
    "pids=$(pidof gdbserver 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then kill $pids 2>/dev/null; fi; " ..
    "killall gdbserver 2>/dev/null; " ..
    "sleep 0.3; " ..
    "nohup gdbserver :%s %s >/tmp/gdbserver.log 2>&1 </dev/null &",
    port, remote_path
  ))
)
  gdbforge.print("starting gdbserver on target …")
  local code, out = run_cmd(start)
  if code ~= 0 then
    gdbforge.print("ERROR: could not start gdbserver: " .. tostring(out))
    return
  end

  gdbforge.print("waiting for " .. addr .. " …")
  if not gdbforge.wait_port(addr, 30) then
    gdbforge.print("ERROR: nothing listening on " .. addr)
    return
  end

  -- >>> ADD THESE LINES HERE <<<
  gdbforge.spawn("ssh", "-o", "BatchMode=yes",
    user .. "@" .. host,
    "tail -n +1 -f /tmp/gdbserver.log")
  gdbforge.print("inferior log: :b exec  (ssh tail -f /tmp/gdbserver.log)")
  -- >>> END ADD <<<
  

  -- 3) target remote  4) break main  5) continue
  gdbforge.open_buffer("gdb")
  gdbforge.gdb("file " .. app)
  gdbforge.gdb("target remote " .. addr)
  gdbforge.gdb("break main")
  gdbforge.gdb("continue")
  gdbforge.print("remotegdb: attached, break main, continue")
end

-- remote_log — open a terminal and tail the board gdbserver log.
--
-- Install:  mkdir -p .gdbforge/lua && cp -r …/lua/remote_log .gdbforge/lua/
-- Usage:    :lua remote_log
--           :lua remote_log 192.168.20.50
--           :lua remote_log 192.168.20.50 root /tmp/gdbserver.log
-- Env:      GDBFORGE_REMOTE_HOST GDBFORGE_REMOTE_USER GDBFORGE_REMOTE_LOG
--           GDBFORGE_TERMINAL=mate-terminal|kitty|xterm|…

local DEFAULT_HOST = "192.168.20.50"
local DEFAULT_USER = "root"
local DEFAULT_LOG  = "/tmp/gdbserver.log"

function help()
  gdbforge.print("remotegdb_log — ssh tail -f board gdbserver log in an external terminal")
  gdbforge.print("Usage: :lua remotegdb_log [host] [user] [logpath]")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("  export GDBFORGE_REMOTE_HOST=" .. DEFAULT_HOST)
  gdbforge.print("  export GDBFORGE_REMOTE_USER=" .. DEFAULT_USER)
  gdbforge.print("  export GDBFORGE_REMOTE_LOG=" .. DEFAULT_LOG)
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

function main(host, user, logpath)
  host = trim(host)
  if host == "" then
    host = env("GDBFORGE_REMOTE_HOST", DEFAULT_HOST)
  end
  user = trim(user)
  if user == "" then
    user = env("GDBFORGE_REMOTE_USER", DEFAULT_USER)
  end
  logpath = trim(logpath)
  if logpath == "" then
    logpath = env("GDBFORGE_REMOTE_LOG", DEFAULT_LOG)
  end

  local target = user .. "@" .. host
  local cmd = "tail -n +1 -f " .. logpath
  gdbforge.print("opening terminal: ssh " .. target .. " " .. cmd)
  gdbforge.spawn_terminal("ssh", "-t", target, cmd)
end

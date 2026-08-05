-- Open the CodeWidget file in gVim (reuse one server; new tab each time).
-- Default location is the CodeWidget blue pointer (SelLine) — same line as
-- j/k browse, whether that sits on ━━▶ PC or elsewhere.
--
-- Install: embedded catalog — or: cp -r lua/gvim .gdbforge/lua/
--
-- Env:
--   GDBFORGE_GVIM_SERVER=GDBFORGE   servername (default GDBFORGE)
--   GDBFORGE_GVIM=gvim              binary (default gvim)
--
-- Usage:
--   :lua gvim                 -- CodeWidget blue pointer line
--   :lua gvim /path/to/f.c    -- explicit path (optional +line as 2nd arg)
--   :lua gvim /path/to/f.c 42
--
-- Uses: gvim --servername NAME --remote-tab-silent +LINE FILE (backgrounded)
-- First call starts the server; later calls open another tab in the same app.
-- Launch is detached so the Lua job returns immediately (does not wait on GUI).

local function env(name, default)
  local v = os.getenv(name)
  if v == nil or v == "" then
    return default
  end
  return v
end

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

-- CodeWidget blue pointer (browse cursor); coincides with PC after a stop.
local function default_loc()
  local path = gdbforge.current_file()
  local ln = tonumber(gdbforge.current_line()) or 1
  if ln < 1 then
    ln = 1
  end
  return path, ln
end

function help()
  gdbforge.print("gvim — open CodeWidget file in gVim (same --servername, new tab)")
  gdbforge.print("Usage: :lua gvim [file] [line]")
  gdbforge.print("  :lua gvim              -- blue pointer line (PC or browse)")
  gdbforge.print("  :lua gvim hello.c 10   -- explicit path + line")
  gdbforge.print("Env:")
  gdbforge.print("  GDBFORGE_GVIM_SERVER=GDBFORGE")
  gdbforge.print("  GDBFORGE_GVIM=gvim")
end

function main(file, line)
  local path = file
  local ln = tonumber(line)
  if path == nil or path == "" then
    path, ln = default_loc()
  elseif ln == nil or ln < 1 then
    ln = tonumber(gdbforge.current_line()) or 1
  end
  if path == nil or path == "" then
    gdbforge.print("gvim: no CodeWidget file — :edit a source first")
    return
  end
  if ln == nil or ln < 1 then
    ln = 1
  end

  local bin = env("GDBFORGE_GVIM", "gvim")
  local server = env("GDBFORGE_GVIM_SERVER", "GDBFORGE")
  -- Detach: gVim is a long-lived GUI; gdbforge.system must not wait on it
  -- (that wedged the Lua job and made Ctrl-C steal interrupt from GDB).
  local cmd = string.format(
    "( %s --servername %s --remote-tab-silent +%d %s >/dev/null 2>&1 & )",
    shell_quote(bin),
    shell_quote(server),
    ln,
    shell_quote(path)
  )
  local status, out = gdbforge.system(cmd)
  if status ~= 0 then
    gdbforge.print("gvim failed (" .. tostring(status) .. "): " .. tostring(out))
    return
  end
  gdbforge.print(string.format("gvim [%s] tab: %s:%d", server, path, ln))
end

-- Open the CodeWidget file in terminal vim on top of gdbforge.
-- Suspends the TUI; when vim exits, gdbforge resumes the terminal.
--
-- Install: embedded catalog — or: cp -r lua/vim .gdbforge/lua/
--
-- Env:
--   GDBFORGE_VIM=vim              binary (default vim; e.g. nvim)
--
-- Usage:
--   :lua vim                  -- CodeWidget blue pointer line
--   :lua vim /path/to/f.c     -- explicit path (optional line as 2nd arg)
--   :lua vim /path/to/f.c 42
--
-- Uses: gdbforge.foreground(VIM, "+LINE", FILE)
-- Blocks until vim quits; then gdbforge redraws and takes the tty again.

local function env(name, default)
  local v = os.getenv(name)
  if v == nil or v == "" then
    return default
  end
  return v
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
  gdbforge.print("vim — open CodeWidget file in terminal vim (over gdbforge)")
  gdbforge.print("Usage: :lua vim [file] [line]")
  gdbforge.print("  :lua vim               -- blue pointer line (PC or browse)")
  gdbforge.print("  :lua vim hello.c 10    -- explicit path + line")
  gdbforge.print("Suspends gdbforge TUI; resumes when vim exits (:q).")
  gdbforge.print("Env:")
  gdbforge.print("  GDBFORGE_VIM=vim|nvim")
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
    gdbforge.print("vim: no CodeWidget file — :edit a source first")
    return
  end
  if ln == nil or ln < 1 then
    ln = 1
  end

  local bin = env("GDBFORGE_VIM", "vim")
  gdbforge.print(string.format("vim: %s +%d %s  (quit vim to return)", bin, ln, path))
  -- Blocks on UI thread via foreground: Suspend → vim → Resume.
  gdbforge.foreground(bin, string.format("+%d", ln), path)
  gdbforge.print(string.format("vim done: %s:%d", path, ln))
end

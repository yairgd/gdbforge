-- Open the CodeWidget file in VS Code or VSCodium (reuse window; goto line).
-- Default location is the CodeWidget blue pointer (SelLine) — same line as
-- j/k browse, whether that sits on ━━▶ PC or elsewhere.
-- Optionally opens the detected project folder as the workspace.
--
-- Install: embedded catalog — or: cp -r lua/vscode .gdbforge/lua/
--
-- Env:
--   GDBFORGE_VSCODE=code|codium          force binary (skip auto-detect)
--   GDBFORGE_VSCODE_WORKSPACE=/path      force workspace folder / .code-workspace
--
-- Usage:
--   :lua vscode                 -- CodeWidget blue pointer line
--   :lua vscode /path/to/f.c    -- explicit path (optional line as 2nd arg)
--   :lua vscode /path/to/f.c 42
--
-- Picks the first available of: code, codium (unless GDBFORGE_VSCODE is set).
-- Workspace root: walk up from the file for .git / .gdbforge / go.mod / CMakeLists.txt
--   (or GDBFORGE_VSCODE_WORKSPACE). If none found, open file only.
-- Uses: BIN -r [WORKSPACE] -g FILE:LINE
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

local function trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
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

-- Prefer explicit override, else first of code / codium on PATH.
local function resolve_bin()
  local override = env("GDBFORGE_VSCODE", "")
  if override ~= "" then
    return override
  end
  local st = gdbforge.system("command -v code >/dev/null 2>&1")
  if st == 0 then
    return "code"
  end
  st = gdbforge.system("command -v codium >/dev/null 2>&1")
  if st == 0 then
    return "codium"
  end
  return nil
end

-- Project root for the editor workspace: env override, else walk parents of file.
local function find_workspace(file)
  local override = env("GDBFORGE_VSCODE_WORKSPACE", "")
  if override ~= "" then
    return override
  end
  if file == nil or file == "" then
    return nil
  end
  -- Walk up looking for common project markers; print first match.
  local cmd = string.format(
    "d=$(dirname %s); " ..
    "while [ \"$d\" != \"/\" ] && [ -n \"$d\" ]; do " ..
    "  if [ -d \"$d/.git\" ] || [ -d \"$d/.gdbforge\" ] " ..
    "     || [ -f \"$d/go.mod\" ] || [ -f \"$d/CMakeLists.txt\" ]; then " ..
    "    printf '%%s' \"$d\"; exit 0; " ..
    "  fi; " ..
    "  d=$(dirname \"$d\"); " ..
    "done; exit 1",
    shell_quote(file)
  )
  local st, out = gdbforge.system(cmd)
  if st ~= 0 then
    return nil
  end
  out = trim(out)
  if out == "" then
    return nil
  end
  return out
end

function help()
  gdbforge.print("vscode — open CodeWidget file in VS Code / VSCodium (reuse window)")
  gdbforge.print("Usage: :lua vscode [file] [line]")
  gdbforge.print("  :lua vscode              -- blue pointer line (PC or browse)")
  gdbforge.print("  :lua vscode hello.c 10   -- explicit path + line")
  gdbforge.print("Detects: code, then codium (first found on PATH)")
  gdbforge.print("Workspace: nearest .git / .gdbforge / go.mod / CMakeLists.txt")
  gdbforge.print("  (falls back to file-only if none found)")
  gdbforge.print("Env:")
  gdbforge.print("  GDBFORGE_VSCODE=code|codium")
  gdbforge.print("  GDBFORGE_VSCODE_WORKSPACE=/path  force folder / .code-workspace")
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
    gdbforge.print("vscode: no CodeWidget file — :edit a source first")
    return
  end
  if ln == nil or ln < 1 then
    ln = 1
  end

  local bin = resolve_bin()
  if bin == nil then
    gdbforge.print("vscode: neither 'code' nor 'codium' found on PATH")
    gdbforge.print("  install VS Code / VSCodium, or set GDBFORGE_VSCODE")
    return
  end

  local workspace = find_workspace(path)
  local goto_arg = string.format("%s:%d", path, ln)

  -- Detach: editor is long-lived; do not block the Lua job on the GUI.
  local cmd
  if workspace ~= nil and workspace ~= "" then
    cmd = string.format(
      "( %s -r %s -g %s >/dev/null 2>&1 & )",
      shell_quote(bin),
      shell_quote(workspace),
      shell_quote(goto_arg)
    )
  else
    cmd = string.format(
      "( %s -r -g %s >/dev/null 2>&1 & )",
      shell_quote(bin),
      shell_quote(goto_arg)
    )
  end

  local status, out = gdbforge.system(cmd)
  if status ~= 0 then
    gdbforge.print("vscode failed (" .. tostring(status) .. "): " .. tostring(out))
    return
  end
  if workspace ~= nil and workspace ~= "" then
    gdbforge.print(string.format("vscode [%s] workspace %s → %s:%d", bin, workspace, path, ln))
  else
    gdbforge.print(string.format("vscode [%s] : %s:%d (no workspace root)", bin, path, ln))
  end
end

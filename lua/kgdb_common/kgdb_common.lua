-- Shared helpers for kgdb_uart / kgdb_net (Lua glue only).
-- Does NOT vendor Linux kernel GDB Python (lx-symbols); that stays with the kernel tree.
--
-- Loaded via dofile from sibling scripts, or :lua kgdb_common help

kgdb_common = kgdb_common or {}

function kgdb_common.trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function kgdb_common.env(name, fallback)
  local v = os.getenv(name)
  if v == nil or kgdb_common.trim(v) == "" then
    return fallback
  end
  return kgdb_common.trim(v)
end

function kgdb_common.shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

function kgdb_common.run_cmd(cmd)
  local code, out = gdbforge.system(cmd)
  return tonumber(code) or 1, out or ""
end

function kgdb_common.read_file(path)
  local code, out = kgdb_common.run_cmd("cat " .. kgdb_common.shell_quote(path) .. " 2>/dev/null")
  if code ~= 0 then
    return nil
  end
  return kgdb_common.trim(out)
end

-- Wait until path exists and is non-empty; return trimmed contents or nil.
function kgdb_common.wait_file(path, timeout_sec)
  timeout_sec = tonumber(timeout_sec) or 10
  local deadline = os.time() + timeout_sec
  while os.time() <= deadline do
    local body = kgdb_common.read_file(path)
    if body and body ~= "" then
      return body
    end
    gdbforge.sleep(0.2)
  end
  return nil
end

local function ssh_sysfs_sections(user, host, module_name)
  local code, out = kgdb_common.run_cmd(string.format(
    "ssh -o BatchMode=yes -o ConnectTimeout=8 %s@%s %s",
    user, host,
    kgdb_common.shell_quote(string.format(
      "printf '%%s\\n' \"$(cat /sys/module/%s/sections/.text 2>/dev/null)\" " ..
      "\"$(cat /sys/module/%s/sections/.data 2>/dev/null)\" " ..
      "\"$(cat /sys/module/%s/sections/.bss 2>/dev/null)\"",
      module_name, module_name, module_name
    ))
  ))
  if code ~= 0 then
    return nil, nil, nil, out
  end
  local lines = {}
  for line in tostring(out):gmatch("[^\r\n]+") do
    lines[#lines + 1] = kgdb_common.trim(line)
  end
  return lines[1], lines[2], lines[3], out
end

--- Load module / kernel symbols after target remote.
-- opts:
--   modules_dir  — passed to lx-symbols (optional)
--   skip_lx      — if true, skip lx-symbols
--   ko_path      — local .ko for add-symbol-file
--   text, data, bss — addresses (strings) for add-symbol-file
--   ssh_user, ssh_host, module_name — optional sysfs fetch (path 2)
function kgdb_common.load_symbols(opts)
  opts = opts or {}

  if not opts.skip_lx then
    local mods = kgdb_common.trim(opts.modules_dir or "")
    if mods ~= "" then
      gdbforge.print("lx-symbols " .. mods)
      gdbforge.gdb("lx-symbols " .. mods)
    else
      gdbforge.print("lx-symbols")
      gdbforge.gdb("lx-symbols")
    end
  end

  local text = kgdb_common.trim(opts.text or "")
  local data = kgdb_common.trim(opts.data or "")
  local bss = kgdb_common.trim(opts.bss or "")
  local ko = kgdb_common.trim(opts.ko_path or "")
  local mod = kgdb_common.trim(opts.module_name or "")
  local user = kgdb_common.trim(opts.ssh_user or "")
  local host = kgdb_common.trim(opts.ssh_host or "")

  if text == "" and ko ~= "" and mod ~= "" and user ~= "" and host ~= "" then
    gdbforge.print("reading /sys/module/" .. mod .. "/sections via ssh …")
    local t, d, b, err = ssh_sysfs_sections(user, host, mod)
    if not t or t == "" then
      gdbforge.print("WARN: ssh sysfs read failed — skip add-symbol-file")
      if err and kgdb_common.trim(err) ~= "" then
        gdbforge.print(kgdb_common.trim(err))
      end
    else
      text, data, bss = t, d or "", b or ""
    end
  end

  if ko ~= "" and text ~= "" then
    local cmd = "add-symbol-file " .. ko .. " " .. text
    if data ~= "" then
      cmd = cmd .. " -s .data " .. data
    end
    if bss ~= "" then
      cmd = cmd .. " -s .bss " .. bss
    end
    gdbforge.print(cmd)
    gdbforge.gdb(cmd)
  end
end

function help()
  gdbforge.print("kgdb_common — shared helpers for kgdb_uart / kgdb_net (not a workflow)")
  gdbforge.print("Use: :lua kgdb_uart  or  :lua kgdb_net")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main()
  help()
end

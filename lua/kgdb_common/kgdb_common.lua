-- Shared helpers for kgdb_uart / kgdb_net (Lua glue only).
-- Does NOT vendor Linux kernel GDB Python (lx-symbols); that stays with the kernel tree.
--
-- Loaded via dofile from sibling scripts, or :lua kgdb_common help

kgdb_common = kgdb_common or {}

function kgdb_common.trim(s)
  s = tostring(s or "")
  s = s:gsub("^%s+", "")
  s = s:gsub("%s+$", "")
  return s
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

function kgdb_common.file_exists(path)
  local code = kgdb_common.run_cmd("test -e " .. kgdb_common.shell_quote(path))
  return code == 0
end

function kgdb_common.dirname(path)
  return tostring(path or ""):match("^(.*)/[^/]*$") or "."
end

function kgdb_common.pid_alive(pid)
  pid = kgdb_common.trim(pid)
  if pid == "" then
    return false
  end
  local code = kgdb_common.run_cmd("kill -0 " .. pid .. " 2>/dev/null")
  return code == 0
end

--- PIDs currently holding a device, as { {pid=, name=}, … }.
function kgdb_common.device_holders(dev)
  local q = kgdb_common.shell_quote(dev)
  local _, out = kgdb_common.run_cmd("fuser " .. q .. " 2>/dev/null || lsof -t " .. q .. " 2>/dev/null")
  local list, seen = {}, {}
  for pid in tostring(out or ""):gmatch("%d+") do
    if not seen[pid] then
      seen[pid] = true
      local _, comm = kgdb_common.run_cmd("ps -o comm= -p " .. pid .. " 2>/dev/null")
      list[#list + 1] = { pid = pid, name = kgdb_common.trim(comm) }
    end
  end
  return list
end

-- Serial console programs (and stale helpers) that must not share the UART.
local reclaimable = {
  kdmx = true,
  minicom = true,
  picocom = true,
  screen = true,
  socat = true,
  tio = true,
  cu = true,
  ["agent-proxy"] = true,
  gdbforge = true,
  xterm = true,
  ["mate-terminal"] = true,
  ["gnome-terminal"] = true,
  kitty = true,
  konsole = true,
  alacritty = true,
  sh = true,
  bash = true,
  cat = true,
  tee = true,
  xclip = true,
}

--- Make `dev` exclusively available for kdmx.
-- mode comes from GDBFORGE_KGDB_TAKEOVER:
--   auto  (default) — terminate known serial consoles (minicom, picocom, …);
--                     any other holder aborts the run
--   force           — terminate whatever holds the device
--   never           — never kill; abort while the device is held
function kgdb_common.free_device(dev, mode)
  -- Earlier callers passed a boolean force flag.
  if mode == true then
    mode = "force"
  elseif mode == false or mode == nil then
    mode = "auto"
  end
  mode = tostring(mode):lower()

  local holders = kgdb_common.device_holders(dev)
  if #holders == 0 then
    return true
  end

  local blocked, killed = {}, {}
  for _, h in ipairs(holders) do
    if mode ~= "never" and (mode == "force" or reclaimable[h.name]) then
      gdbforge.print("releasing " .. dev .. ": terminating " .. h.name .. " (pid " .. h.pid .. ")")
      kgdb_common.run_cmd("kill " .. h.pid .. " 2>/dev/null")
      killed[#killed + 1] = h.pid
    else
      blocked[#blocked + 1] = h.name .. " (pid " .. h.pid .. ")"
    end
  end

  if #blocked > 0 then
    gdbforge.print("ERROR: " .. dev .. " is held by " .. table.concat(blocked, ", "))
    gdbforge.print("Close the program above, or reclaim the port:")
    gdbforge.print("  export GDBFORGE_KGDB_TAKEOVER=force")
    gdbforge.print("  fuser -k " .. dev .. "   # last resort")
    return false
  end

  for _ = 1, 20 do
    gdbforge.sleep(0.25)
    if #kgdb_common.device_holders(dev) == 0 then
      return true
    end
  end

  for _, pid in ipairs(killed) do
    kgdb_common.run_cmd("kill -9 " .. pid .. " 2>/dev/null")
  end
  gdbforge.sleep(0.5)

  if #kgdb_common.device_holders(dev) > 0 then
    gdbforge.print("ERROR: " .. dev .. " still busy after kill")
    return false
  end
  return true
end

-- Upstream kdmx exits when a read of the serial fd returns EAGAIN, which tears
-- down both PTYs and surfaces in GDB as "Remote connection closed". The build
-- shipped in gdbforge's bin/ retries instead, and marks itself in -v output.
local kdmx_patch_marker = "gdbforge"

--- Pick the kdmx binary: explicit override, then gdbforge's bin/, then PATH.
function kgdb_common.resolve_kdmx(explicit, lua_dir)
  explicit = kgdb_common.trim(explicit or "")
  if explicit ~= "" then
    return explicit
  end

  local candidates = {}
  lua_dir = kgdb_common.trim(lua_dir or "")
  if lua_dir ~= "" then
    -- .gdbforge/lua/kgdb_uart/ and lua/kgdb_uart/ respectively.
    candidates[#candidates + 1] = lua_dir .. "/../../../bin/kdmx"
    candidates[#candidates + 1] = lua_dir .. "/../../bin/kdmx"
  end
  candidates[#candidates + 1] = "./bin/kdmx"

  for _, path in ipairs(candidates) do
    if kgdb_common.file_exists(path) then
      return path
    end
  end
  return "kdmx"
end

--- Report whether `path` is the patched kdmx. Returns ok, version.
function kgdb_common.check_kdmx(path)
  local code, out = kgdb_common.run_cmd(kgdb_common.shell_quote(path) .. " -v 2>&1")
  local ver = kgdb_common.trim(out)
  if code ~= 0 then
    return false, (ver ~= "" and ver or "cannot execute " .. path)
  end
  return ver:find(kdmx_patch_marker, 1, true) ~= nil, ver
end

--- Report whether the session's GDB can run Python.
-- Without Python, "source vmlinux-gdb.py" is silently parsed as a GDB command
-- file (script-extension defaults to "soft"), whose first failure is the
-- confusing `Undefined command: "import"`. Detect that up front instead.
-- Returns ok, detail.
function kgdb_common.gdb_python_ok()
  local gdb_bin = ""
  if gdbforge.debugger_path then
    gdb_bin = kgdb_common.trim(gdbforge.debugger_path())
  end
  if gdb_bin == "" then
    gdb_bin = "gdb"
  end

  local code, out = kgdb_common.run_cmd(string.format(
    "%s -nx --batch -ex %s 2>&1",
    kgdb_common.shell_quote(gdb_bin),
    kgdb_common.shell_quote('python print("GDBFORGE_PY_OK")')
  ))
  out = tostring(out or "")
  if code == 0 and out:find("GDBFORGE_PY_OK", 1, true) then
    return true, gdb_bin
  end

  -- Prefer the line that names the cause over Python's path dump.
  local detail
  for _, pattern in ipairs({
    "Python initialization failed[^\r\n]*",
    "Python not initialized[^\r\n]*",
    "[^\r\n]*not supported[^\r\n]*",
    "Undefined command[^\r\n]*",
  }) do
    detail = out:match(pattern)
    if detail then
      break
    end
  end
  detail = detail or out:match("[^\r\n]*[Pp]ython[^\r\n]*") or out
  return false, gdb_bin .. ": " .. kgdb_common.trim(detail)
end

--- Source the kernel's own GDB helpers so lx-symbols exists.
-- Not vendored: located next to vmlinux, in the build tree, or via
-- GDBFORGE_KGDB_SCRIPTS. Returns true when a script was sourced.
function kgdb_common.source_kernel_scripts(scripts, vmlinux, modules_dir)
  local candidates = {}
  scripts = kgdb_common.trim(scripts or "")
  if scripts ~= "" then
    candidates[#candidates + 1] = scripts
    candidates[#candidates + 1] = scripts .. "/vmlinux-gdb.py"
    candidates[#candidates + 1] = scripts .. "/scripts/gdb/vmlinux-gdb.py"
  end
  vmlinux = kgdb_common.trim(vmlinux or "")
  if vmlinux ~= "" then
    candidates[#candidates + 1] = kgdb_common.dirname(vmlinux) .. "/vmlinux-gdb.py"
  end
  modules_dir = kgdb_common.trim(modules_dir or "")
  if modules_dir ~= "" then
    candidates[#candidates + 1] = modules_dir .. "/vmlinux-gdb.py"
    candidates[#candidates + 1] = modules_dir .. "/scripts/gdb/vmlinux-gdb.py"
  end

  for _, path in ipairs(candidates) do
    if path:match("%.py$") and kgdb_common.file_exists(path) then
      local py_ok, detail = kgdb_common.gdb_python_ok()
      if not py_ok then
        gdbforge.print("ERROR: this GDB cannot run Python — skipping " .. path)
        gdbforge.print("  " .. tostring(detail))
        gdbforge.print("lx-symbols needs a Python-enabled GDB. Without it, GDB would")
        gdbforge.print("parse the script as commands and fail on: Undefined command: \"import\"")
        gdbforge.print("Fixes:")
        gdbforge.print("  unset PYTHONHOME PYTHONPATH   # a bad PYTHONHOME disables GDB python")
        gdbforge.print("  gdbforge -g gdb -d /path/to/python-enabled-gdb")
        return false
      end
      gdbforge.print("add-auto-load-safe-path " .. kgdb_common.dirname(path))
      gdbforge.gdb("add-auto-load-safe-path " .. kgdb_common.dirname(path))
      gdbforge.print("source " .. path)
      gdbforge.gdb("source " .. path)
      return true
    end
  end

  gdbforge.print("WARN: vmlinux-gdb.py not found — lx-symbols will be undefined")
  gdbforge.print("  set GDBFORGE_KGDB_SCRIPTS=/path/to/kernel-source (build tree with scripts/gdb)")
  return false
end

kgdb_common.kgdb_default_kernel_tree = "/home/yair/merlin/kernel-source"

function kgdb_common.kgdb_vmlinux()
  return kgdb_common.env("GDBFORGE_KGDB_VMLINUX",
    kgdb_common.kgdb_default_kernel_tree .. "/vmlinux")
end

function kgdb_common.kgdb_kernel_tree()
  return kgdb_common.env("GDBFORGE_KGDB_SCRIPTS",
    kgdb_common.env("GDBFORGE_KGDB_MODULES", kgdb_common.kgdb_default_kernel_tree))
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

function kgdb_common.load_symbols(opts)
  opts = opts or {}

  if not opts.skip_lx then
    kgdb_common.source_kernel_scripts(opts.scripts, opts.vmlinux, opts.modules_dir)
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

--- Send GDB CLI and wait for (gdb) prompt. Needs gdbforge.gdb_query (rebuild after update).
function kgdb_common.gdb_query(cmd, timeout_sec)
  cmd = kgdb_common.trim(cmd or "")
  if cmd == "" then
    return false, "empty gdb command"
  end
  if not gdbforge.gdb_query then
    gdbforge.gdb(cmd)
    gdbforge.sleep(1)
    return true, ""
  end
  local out, err = gdbforge.gdb_query(cmd, timeout_sec or 120)
  out = tostring(out or "")
  err = err and tostring(err) or ""
  if err ~= "" then
    return false, err
  end
  local line = out:match("([^\r\n]+)")
  if line and (line:find("No symbol", 1, true) or line:find("Undefined command", 1, true)) then
    return false, line
  end
  return true, out
end

function kgdb_common.gdb_target_remote(gdb_pty, timeout_sec)
  gdb_pty = kgdb_common.trim(gdb_pty or "")
  if gdb_pty == "" then
    return false, "empty gdb PTY path"
  end
  return kgdb_common.gdb_query("target remote " .. gdb_pty, timeout_sec or 120)
end

--- Enter kgdb on the target via SysRq-G on the shared UART (serial mux).
function kgdb_common.kgdb_serial_sysrq()
  if not gdbforge.serial_send then
    return false, "serial_send not available — run :lua kgdb_serial first"
  end
  gdbforge.print("serial: echo g > /proc/sysrq-trigger")
  local ok, err = pcall(gdbforge.serial_send, "echo g > /proc/sysrq-trigger")
  if not ok then
    return false, tostring(err)
  end
  gdbforge.print("sysrq-g sent — kernel should break in on serial")
  return true, nil
end

--- First attach: target remote (blocks) with delayed sysrq-g on the UART.
function kgdb_common.kgdb_serial_attach(gdb_pty, opts)
  opts = opts or {}
  gdb_pty = kgdb_common.trim(gdb_pty or "")
  if gdb_pty == "" then
    return false, "empty gdb PTY path"
  end

  local attach_wait = tonumber(opts.attach_wait) or 120
  local sysrq_delay = tonumber(opts.sysrq_delay) or 1
  local do_sysrq = opts.sysrq ~= false

  if do_sysrq then
    gdbforge.print("attach: target remote " .. gdb_pty .. " + sysrq-g in " ..
      sysrq_delay .. "s (parallel)")
    if gdbforge.serial_sysrq_delayed then
      gdbforge.serial_sysrq_delayed(sysrq_delay)
    else
      gdbforge.sleep(sysrq_delay)
      local ok, err = kgdb_common.kgdb_serial_sysrq()
      if not ok then
        return false, err
      end
    end
  else
    gdbforge.print("target remote " .. gdb_pty .. " (no sysrq — board must already be in kgdb)")
  end

  return kgdb_common.gdb_target_remote(gdb_pty, attach_wait)
end

function kgdb_common.kgdb_serial_state_path()
  local home = os.getenv("HOME") or ""
  return home .. "/.cache/gdbforge/kgdb_serial.state"
end

function kgdb_common.kgdb_serial_clear_state()
  os.remove(kgdb_common.kgdb_serial_state_path())
end

function help()
  gdbforge.print("kgdb_common — shared helpers for kgdb_uart / kgdb_net (not a workflow)")
  gdbforge.print("Use: :lua kgdb_uart  or  :lua kgdb_net")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main()
  help()
end

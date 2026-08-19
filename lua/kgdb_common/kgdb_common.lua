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

--- OpenSSH options for board automation. Override with GDBFORGE_SSH_OPTS.
function kgdb_common.ssh_opts()
  local extra = kgdb_common.env("GDBFORGE_SSH_OPTS", "")
  if extra ~= "" then
    return extra
  end
  -- LogLevel=ERROR hides OpenSSH 9+/10 post-quantum KEX warnings on stderr.
  return "-o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR"
end

--- Drop ssh client noise (PQ warnings, blank lines) from captured output.
function kgdb_common.ssh_sanitize(out)
  local lines = {}
  for line in tostring(out or ""):gmatch("[^\r\n]+") do
    line = kgdb_common.trim(line)
    if line ~= "" and not line:match("^%*%*") then
      lines[#lines + 1] = line
    end
  end
  if #lines == 0 then
    return ""
  end
  return table.concat(lines, "\n")
end

--- Run a remote shell command over ssh. Returns exit code, combined output.
function kgdb_common.ssh_run(user, host, remote_cmd)
  user = kgdb_common.trim(user or "")
  host = kgdb_common.trim(host or "")
  if user == "" or host == "" then
    return 1, "ssh user/host required"
  end
  local code, out = kgdb_common.run_cmd(string.format(
    "ssh %s %s@%s %s",
    kgdb_common.ssh_opts(), user, host, kgdb_common.shell_quote(remote_cmd)
  ))
  return code, kgdb_common.ssh_sanitize(out)
end

--- Safe board-side TTY name for kgdboc (no /dev/ prefix).
function kgdb_common.normalize_board_tty(tty)
  tty = kgdb_common.trim(tty or "")
  tty = tty:gsub("^/dev/", "")
  if tty == "" or not tty:match("^[%w%.]+$") then
    return nil
  end
  return tty
end

--- Guess the board kgdb/console tty via SSH (console= or console/active).
function kgdb_common.detect_board_kgdb_tty(user, host)
  local code, out = kgdb_common.ssh_run(user, host, [[
active=$(cat /sys/class/tty/console/active 2>/dev/null | awk '{print $1}')
if [ -n "$active" ]; then
  echo "$active"
  exit 0
fi
grep -oE 'console=[^ ]+' /proc/cmdline 2>/dev/null | head -1 | sed 's/console=//' | cut -d, -f1
]])
  if code ~= 0 then
    return nil, kgdb_common.trim(out)
  end
  local tty = kgdb_common.normalize_board_tty(out:match("[^\r\n]+"))
  if not tty then
    return nil, "could not parse board console tty"
  end
  return tty, nil
end

--- Configure kgdboc on the target over SSH. Returns ok, detail.
function kgdb_common.setup_kgdboc(user, host, board_tty, baud)
  board_tty = kgdb_common.normalize_board_tty(board_tty)
  if not board_tty then
    return false, "invalid board tty"
  end
  baud = kgdb_common.trim(baud or "115200")
  local val = board_tty .. "," .. baud
  local _, cur = kgdb_common.ssh_run(user, host,
    "cat /sys/module/kgdboc/parameters/kgdboc 2>/dev/null"
  )
  cur = kgdb_common.trim(cur)
  if cur == val then
    gdbforge.print("target kgdboc: " .. cur .. " (unchanged)")
    return true, cur
  end

  gdbforge.print("ssh: kgdboc=" .. val .. " on " .. kgdb_common.kgdb_ssh_target(user, host))
  local code, out = kgdb_common.ssh_run(user, host, string.format(
    "if echo %s > /sys/module/kgdboc/parameters/kgdboc 2>/dev/null; then exit 0; fi; " ..
    "echo \"kgdboc write failed for %s\" >&2; " ..
    "cat /sys/module/kgdboc/parameters/kgdboc 2>/dev/null; exit 1",
    val, board_tty
  ))
  if code ~= 0 then
    local detail = kgdb_common.trim(out)
    if detail == "" then
      detail = "kgdboc write failed for " .. board_tty .. " (No such device?)"
    end
    return false, detail .. " — try export GDBFORGE_KGDB_BOARD_TTY=ttyPS0"
  end
  local _, cur = kgdb_common.ssh_run(user, host,
    "cat /sys/module/kgdboc/parameters/kgdboc 2>/dev/null"
  )
  cur = kgdb_common.trim(cur)
  if cur == "" then
    return false, "kgdboc parameter empty (CONFIG_KGDB_SERIAL_CONSOLE missing?)"
  end
  if not cur:find(board_tty, 1, true) then
    return false, "kgdboc now '" .. cur .. "' (expected " .. board_tty .. ")"
  end
  gdbforge.print("target kgdboc: " .. cur)
  return true, cur
end

--- Send one shell line on the raw host UART (before kdmx). Needs gdbforge.uart_send.
function kgdb_common.uart_send_line(uart, baud, line)
  uart = kgdb_common.trim(uart or "")
  line = kgdb_common.trim(line or "")
  if uart == "" or line == "" then
    return false, "uart_send: empty device or command"
  end
  if not gdbforge.uart_send then
    return false, "uart_send not available — rebuild gdbforge"
  end
  local ok, err = pcall(gdbforge.uart_send, uart, tonumber(baud) or 115200, line)
  if not ok then
    return false, tostring(err)
  end
  return true, nil
end

--- Configure kgdboc on the board over the raw UART (board must show a shell prompt).
function kgdb_common.setup_kgdboc_uart(uart, baud, board_tty)
  board_tty = kgdb_common.normalize_board_tty(board_tty)
  if not board_tty then
    return false, "invalid board tty"
  end
  baud = kgdb_common.trim(baud or "115200")
  local val = board_tty .. "," .. baud
  gdbforge.print("uart: echo " .. val .. " > /sys/module/kgdboc/parameters/kgdboc")
  return kgdb_common.uart_send_line(uart, baud,
    "echo " .. val .. " > /sys/module/kgdboc/parameters/kgdboc")
end

--- Shell alias for sysrq-g on the board (bash; use `source ~/.bashrc` in sh).
kgdb_common.brk_alias_line = "alias brk='echo g > /proc/sysrq-trigger'"

--- Define brk alias on the board over the raw UART (does not run brk).
function kgdb_common.install_brk_alias_uart(uart, baud)
  uart = kgdb_common.trim(uart or "")
  local line = kgdb_common.brk_alias_line
  gdbforge.print("uart: " .. line .. " on " .. uart)
  local ok, detail = kgdb_common.uart_send_line(uart, baud,
    "grep -qF " .. kgdb_common.shell_quote(line) .. " /root/.bashrc 2>/dev/null || " ..
    "echo " .. kgdb_common.shell_quote(line) .. " >> /root/.bashrc")
  if not ok then
    return false, detail
  end
  ok, detail = kgdb_common.uart_send_line(uart, baud, line)
  if not ok then
    return false, detail
  end
  gdbforge.print("board shell (bash): brk  — or: source ~/.bashrc")
  return true, nil
end

--- Run brk (sysrq-g) on the board over UART or kdmx console PTY.
function kgdb_common.run_brk_uart(device, baud)
  device = kgdb_common.trim(device or "")
  if device == "" then
    return false, "empty uart/console device"
  end
  gdbforge.print("uart: brk")
  local ok, detail = kgdb_common.uart_send_line(device, baud, "brk")
  if not ok then
    gdbforge.print("WARN: brk — " .. tostring(detail) .. " (trying sysrq directly)")
    ok, detail = kgdb_common.uart_send_line(device, baud, "echo g > /proc/sysrq-trigger")
    if not ok then
      return false, detail
    end
  end
  gdbforge.print("sysrq-g sent — kernel should break in on serial")
  return true, nil
end

--- kgdboc + brk alias on the board UART before kdmx (no SSH).
function kgdb_common.setup_board_before_kdmx(uart, baud, board_tty)
  local ok, detail = kgdb_common.setup_kgdboc_uart(uart, baud, board_tty)
  if not ok then
    return false, detail
  end
  ok, detail = kgdb_common.install_brk_alias_uart(uart, baud)
  if not ok then
    gdbforge.print("WARN: brk alias — " .. tostring(detail))
  end
  return true, nil
end

--- target remote after brk on the kdmx console PTY (serial only, no SSH).
function kgdb_common.gdb_attach_with_uart_brk(gdb_pty, console_pty, opts)
  opts = opts or {}
  gdb_pty = kgdb_common.trim(gdb_pty or "")
  if gdb_pty == "" then
    return false, "empty gdb PTY path"
  end

  local attach_wait = tonumber(opts.attach_wait) or 120
  local brk_delay = tonumber(opts.brk_delay) or tonumber(opts.sysrq_delay) or 0.5
  local do_brk = opts.brk ~= false

  if do_brk then
    local dev = kgdb_common.trim(console_pty or opts.uart or "")
    if dev == "" then
      return false, "console PTY required for uart brk"
    end
    gdbforge.print("attach: brk on " .. dev .. ", wait " .. brk_delay .. "s, then target remote")
    local ok, detail = kgdb_common.run_brk_uart(dev, opts.baud)
    if not ok then
      return false, detail
    end
    gdbforge.sleep(brk_delay)
  else
    gdbforge.print("target remote " .. gdb_pty .. " (no brk — board must already be in kgdb)")
  end

  return kgdb_common.gdb_target_remote(gdb_pty, attach_wait)
end

--- Fixed kdmx status file prefix (creates ${prefix}_gdb and ${prefix}_trm).
kgdb_common.kdmx_status_prefix = "/tmp/kdmx_ports"

function kgdb_common.kgdb_vmlinux()
  return kgdb_common.env("GDBFORGE_KGDB_VMLINUX", "")
end

function kgdb_common.kgdb_kernel_tree()
  return kgdb_common.env("GDBFORGE_KGDB_SCRIPTS",
    kgdb_common.env("GDBFORGE_KGDB_MODULES", ""))
end

--- Board IP for kgdb SSH (never uses GDBFORGE_REMOTE_HOST — that may be a hostname).
kgdb_common.kgdb_default_host = "192.168.20.50"

function kgdb_common.kgdb_host()
  return kgdb_common.env("GDBFORGE_KGDB_HOST", kgdb_common.kgdb_default_host)
end

function kgdb_common.kgdb_user()
  return kgdb_common.env("GDBFORGE_KGDB_SSH_USER",
    kgdb_common.env("GDBFORGE_REMOTE_USER", "root"))
end

function kgdb_common.kgdb_ssh_target(user, host)
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  return user .. "@" .. host
end

--- Board-side kgdboc tty (default ttyPS0; corrects common ttyS0 mistake on Zynq).
function kgdb_common.resolve_board_tty(fallback)
  fallback = kgdb_common.normalize_board_tty(fallback or "ttyPS0") or "ttyPS0"
  local v = kgdb_common.env("GDBFORGE_KGDB_BOARD_TTY", fallback)
  v = kgdb_common.normalize_board_tty(v) or fallback
  if v == "ttyS0" and fallback == "ttyPS0" then
    gdbforge.print("WARN: GDBFORGE_KGDB_BOARD_TTY=ttyS0 ignored — using ttyPS0")
    gdbforge.print("      unset GDBFORGE_KGDB_BOARD_TTY to silence")
    return "ttyPS0"
  end
  return v
end

--- Host-side gdb binary for batch cleanup.
function kgdb_common.gdb_bin()
  if gdbforge.debugger_path then
    local p = kgdb_common.trim(gdbforge.debugger_path())
    if p ~= "" then
      return p
    end
  end
  return "gdb"
end

--- Connect over kdmx gdb PTY and disconnect (releases kernel kgdb_connected).
function kgdb_common.gdb_batch_disconnect(uart, baud, kdmx)
  uart = kgdb_common.trim(uart or "")
  if uart == "" or not kgdb_common.file_exists(uart) then
    return false
  end
  baud = kgdb_common.trim(baud or "115200")
  kdmx = kdmx or "kdmx"
  local status = kgdb_common.kdmx_status_prefix .. "_cleanup"
  kgdb_common.kill_kdmx(uart)
  kgdb_common.free_device(uart, "auto")

  local gdb_pty, _, _, _ = kgdb_common.start_kdmx({
    uart = uart,
    baud = baud,
    kdmx = kdmx,
    takeover = "auto",
    status_prefix = status,
    retries = 1,
  })
  if not gdb_pty then
    return false
  end
  gdb_pty = kgdb_common.read_file(status .. "_gdb") or gdb_pty

  gdbforge.print("cleanup: batch gdb disconnect on " .. gdb_pty)
  local cmd = string.format(
    "%s -batch -nx -ex %s -ex disconnect -ex quit 2>&1",
    kgdb_common.shell_quote(kgdb_common.gdb_bin()),
    kgdb_common.shell_quote("target remote " .. gdb_pty)
  )
  kgdb_common.run_cmd(cmd)
  gdbforge.sleep(0.5)
  kgdb_common.kill_kdmx(uart)
  return true
end

--- Drop stale GDB remote session (ignore errors if not connected).
function kgdb_common.gdb_disconnect()
  gdbforge.print("gdb: disconnect")
  local ok, err = pcall(function()
    gdbforge.gdb("disconnect")
  end)
  if not ok then
    gdbforge.print("  (not connected)")
  end
  gdbforge.sleep(0.5)
end

--- Stop kdmx and remove status PTY files. Pass uart to kill the mux on that device.
function kgdb_common.kill_kdmx(uart)
  uart = kgdb_common.trim(uart or "")

  local function term_pids(pids)
    for pid in tostring(pids or ""):gmatch("%d+") do
      kgdb_common.run_cmd("kill -TERM " .. pid .. " 2>/dev/null")
    end
  end

  local function kill_pids(pids)
    for pid in tostring(pids or ""):gmatch("%d+") do
      kgdb_common.run_cmd("kill -KILL " .. pid .. " 2>/dev/null")
    end
  end

  -- [k]dmx avoids pkill matching its own argv.
  local _, all = kgdb_common.run_cmd("pgrep -f '[k]dmx' 2>/dev/null")
  term_pids(all)
  if uart ~= "" then
    local _, on_uart = kgdb_common.run_cmd(
      "pgrep -f '[k]dmx.*" .. uart:gsub("/", "\\/") .. "' 2>/dev/null")
    term_pids(on_uart)
    for _, h in ipairs(kgdb_common.device_holders(uart)) do
      if h.name == "kdmx" then
        gdbforge.print("stop kdmx on " .. uart .. " (pid " .. h.pid .. ")")
        kgdb_common.run_cmd("kill -TERM " .. h.pid .. " 2>/dev/null")
      end
    end
  end
  kgdb_common.run_cmd("pkill -TERM -x kdmx 2>/dev/null")
  gdbforge.sleep(0.4)

  _, all = kgdb_common.run_cmd("pgrep -f '[k]dmx' 2>/dev/null")
  kill_pids(all)
  kgdb_common.run_cmd("pkill -KILL -x kdmx 2>/dev/null")

  local prefix = kgdb_common.kdmx_status_prefix
  kgdb_common.run_cmd("rm -f " .. kgdb_common.shell_quote(prefix .. "_gdb") .. " " ..
    kgdb_common.shell_quote(prefix .. "_trm") .. " " ..
    kgdb_common.shell_quote(prefix .. "_cleanup_gdb") .. " " ..
    kgdb_common.shell_quote(prefix .. "_cleanup_trm") .. " 2>/dev/null")
  gdbforge.sleep(0.3)

  if uart ~= "" then
    for _, h in ipairs(kgdb_common.device_holders(uart)) do
      if h.name == "kdmx" then
        gdbforge.print("WARN: kdmx still holds " .. uart .. " — kill -9 pid " .. h.pid)
        kgdb_common.run_cmd("kill -KILL " .. h.pid .. " 2>/dev/null")
      end
    end
  end
end

--- Unconfigure kgdboc over SSH. Returns ok, detail.
function kgdb_common.clear_kgdboc(user, host)
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  local code, out = kgdb_common.ssh_run(user, host,
    "if echo '' > /sys/module/kgdboc/parameters/kgdboc 2>/dev/null; then exit 0; fi; " ..
    "dmesg | tail -1; exit 1"
  )
  return code == 0, kgdb_common.trim(out)
end

--- Tear down previous kgdb session: GDB disconnect, kdmx, kgdboc clear.
function kgdb_common.kgdb_cleanup(opts)
  opts = opts or {}
  local user = opts.user or kgdb_common.kgdb_user()
  local host = opts.host or kgdb_common.kgdb_host()

  gdbforge.print("cleanup: previous kgdb session …")
  if not opts.skip_gdb then
    kgdb_common.gdb_disconnect()
  end

  gdbforge.print("cleanup: stop kdmx")
  kgdb_common.kill_kdmx(opts.uart)

  if opts.skip_ssh then
    return true
  end

  gdbforge.print("cleanup: clear kgdboc on " .. kgdb_common.kgdb_ssh_target(user, host))
  for attempt = 1, 3 do
    local ok, detail = kgdb_common.clear_kgdboc(user, host)
    if ok then
      gdbforge.print("cleanup: kgdboc cleared")
      return true
    end
    if detail:find("Cannot reconfigure", 1, true) or detail:find("busy", 1, true) then
      gdbforge.print("cleanup: kgdb still connected (attempt " .. attempt .. "/3) …")
      if opts.uart and opts.uart ~= "" then
        kgdb_common.gdb_batch_disconnect(opts.uart, opts.baud, opts.kdmx)
      elseif not opts.skip_gdb then
        kgdb_common.gdb_disconnect()
        kgdb_common.kill_kdmx(opts.uart)
      end
      gdbforge.sleep(1)
    else
      if detail ~= "" then
        gdbforge.print("cleanup: " .. detail)
      end
      break
    end
  end
  return false
end

--- GDB settings for kgdb over UART. Baud is set by kdmx/kgdboc on the wire — not via GDB.
function kgdb_common.kgdb_remote_tune(baud)
  baud = kgdb_common.trim(baud or "115200")
  gdbforge.gdb("set remotetimeout 60")
  gdbforge.gdb("set pagination off")
end

--- Send GDB CLI and wait for (gdb) prompt. Needs gdbforge with gdb_query (rebuild after update).
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
  return kgdb_common.gdb_query("target remote " .. gdb_pty, timeout_sec or 60)
end

--- Enter kgdb on the target via SysRq-G over SSH.
function kgdb_common.kgdb_sysrq_trigger(user, host)
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  gdbforge.print("ssh: echo g > /proc/sysrq-trigger on " .. kgdb_common.kgdb_ssh_target(user, host))
  local code, out = kgdb_common.ssh_run(user, host,
    "echo g > /proc/sysrq-trigger 2>&1"
  )
  if code ~= 0 then
    return false, kgdb_common.trim(out)
  end
  gdbforge.print("sysrq-g sent — kernel should break in on serial (kdb> or Entering KGDB)")
  gdbforge.print("  if minicom shows [1]kdb>, GDB target remote switches to kgdb automatically")
  return true, nil
end

--- Runtime attach: GDB target remote first (blocks), sysrq-g in parallel.
-- Minicom may show [1]kdb> when CONFIG_KGDB_KDB=y; GDB remote protocol still attaches.
function kgdb_common.gdb_attach_with_sysrq(gdb_pty, opts)
  opts = opts or {}
  gdb_pty = kgdb_common.trim(gdb_pty or "")
  if gdb_pty == "" then
    return false, "empty gdb PTY path"
  end

  local attach_wait = tonumber(opts.attach_wait) or 120
  local sysrq_delay = tonumber(opts.sysrq_delay) or 1
  local do_sysrq = opts.sysrq ~= false
  local user = opts.user or kgdb_common.kgdb_user()
  local host = opts.host or kgdb_common.kgdb_host()

  if do_sysrq then
    gdbforge.print("step attach: target remote " .. gdb_pty .. " + sysrq-g in " ..
      sysrq_delay .. "s (parallel)")
    gdbforge.print("  minicom may show [1]kdb> — OK; GDB switches kdb→kgdb on connect")
    local bg = string.format(
      "sleep %s && ssh %s %s@%s %s &",
      sysrq_delay,
      kgdb_common.ssh_opts(), user, host,
      kgdb_common.shell_quote("echo g > /proc/sysrq-trigger 2>&1")
    )
    kgdb_common.run_cmd(bg)
  else
    gdbforge.print("target remote " .. gdb_pty .. " (no sysrq — board must already be in kgdb)")
  end

  return kgdb_common.gdb_target_remote(gdb_pty, attach_wait)
end

--- Start kdmx on a host UART. Returns gdb_pty, trm_pty, pid, log or nil, err.
function kgdb_common.start_kdmx(opts)
  opts = opts or {}
  local uart = kgdb_common.trim(opts.uart or "")
  if uart == "" then
    return nil, "uart required"
  end
  if not kgdb_common.file_exists(uart) then
    return nil, "no such device: " .. uart
  end

  local baud = kgdb_common.trim(opts.baud or "115200")
  local kdmx = opts.kdmx or "kdmx"
  local takeover = kgdb_common.trim(opts.takeover or "auto"):lower()
  local retries = tonumber(opts.retries) or 3
  local status = kgdb_common.trim(opts.status_prefix or kgdb_common.kdmx_status_prefix)
  local status_gdb, status_trm = status .. "_gdb", status .. "_trm"

  local kdmx_ok, kdmx_ver = kgdb_common.check_kdmx(kdmx)
  gdbforge.print("kdmx: " .. kdmx .. "  (" .. kdmx_ver .. ")")
  if not kdmx_ok then
    gdbforge.print("WARN: this kdmx exits when a serial read returns EAGAIN, which")
    gdbforge.print("      closes both PTYs — GDB then reports 'Remote connection closed'.")
    gdbforge.print("      Build the patched one, or point at it explicitly:")
    gdbforge.print("        export GDBFORGE_KGDB_KDMX=/path/to/gdbforge/bin/kdmx")
  end

  kgdb_common.kill_kdmx(uart)

  gdbforge.print("claiming " .. uart .. " for kdmx (takeover=" .. takeover .. ") …")
  if not kgdb_common.free_device(uart, takeover) then
    return nil, "could not claim " .. uart
  end

  kgdb_common.run_cmd("rm -f " .. kgdb_common.shell_quote(status_gdb) .. " " ..
    kgdb_common.shell_quote(status_trm) .. " 2>/dev/null")

  local log = "/tmp/kdmx.log"
  local gdb_pty, trm_pty, pid

  for attempt = 1, math.max(1, retries) do
    local start = string.format(
      "nohup %s -n -p %s -b %s -s %s >%s 2>&1 </dev/null & echo $!",
      kgdb_common.shell_quote(kdmx), kgdb_common.shell_quote(uart),
      kgdb_common.shell_quote(baud), kgdb_common.shell_quote(status),
      kgdb_common.shell_quote(log)
    )
    gdbforge.print(string.format("kdmx -n -p %s -b %s -s %s (attempt %d/%d) …",
      uart, baud, status, attempt, math.max(1, retries)))
    local code, out = kgdb_common.run_cmd(start)
    if code ~= 0 then
      return nil, "failed to start kdmx: " .. kgdb_common.trim(out)
    end
    pid = kgdb_common.trim(out):match("(%d+)")

    local g = kgdb_common.wait_file(status_gdb, 15)
    local t = kgdb_common.wait_file(status_trm, 15)
    if g and t and kgdb_common.pid_alive(pid) then
      gdbforge.sleep(1)
      if kgdb_common.pid_alive(pid) then
        gdb_pty = g:match("(/dev/pts/%d+)") or g
        trm_pty = t:match("(/dev/pts/%d+)") or t
        break
      end
    end

    gdbforge.print("kdmx did not come up — log:")
    local _, tail = kgdb_common.run_cmd("tail -5 " .. kgdb_common.shell_quote(log) .. " 2>/dev/null")
    gdbforge.print(kgdb_common.trim(tail))
    if pid then
      kgdb_common.run_cmd("kill -9 " .. pid .. " 2>/dev/null")
    end
    gdbforge.sleep(1)
  end

  if not gdb_pty or not trm_pty then
    return nil, "kdmx failed — check " .. log .. " ('errno 11' → UART held by another program)"
  end

  gdbforge.print("kdmx pid " .. tostring(pid) .. "  (log " .. log .. ")")
  gdbforge.print("console PTY: " .. trm_pty .. "  (" .. status_trm .. ")")
  gdbforge.print("gdb PTY:     " .. gdb_pty .. "  (" .. status_gdb .. ")")
  return gdb_pty, trm_pty, pid, log
end

--- Open minicom on the kdmx console PTY (reads status file if trm_pty omitted).
function kgdb_common.open_serial_console(trm_pty, console_cmd)
  console_cmd = kgdb_common.trim(console_cmd or "minicom")
  local status_trm = kgdb_common.kdmx_status_prefix .. "_trm"
  if console_cmd == "minicom" then
    gdbforge.print("minicom -D $(cat " .. status_trm .. ")")
    if trm_pty and trm_pty ~= "" then
      gdbforge.spawn_terminal("minicom", "-D", trm_pty, "-o")
    else
      gdbforge.spawn_terminal("bash", "-lc",
        "minicom -D \"$(cat " .. status_trm .. ")\" -o")
    end
  else
    gdbforge.print("opening " .. console_cmd .. " on console PTY …")
    gdbforge.spawn_terminal(console_cmd, trm_pty or "")
  end
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

--- Find a built .ko under the kernel tree (GDBFORGE_KGDB_KO overrides).
-- Prefers source-tree paths (drivers/, extra/) over packaged deploy/lib/modules/*.ko.
function kgdb_common.resolve_module_ko(modules_dir, module_name, explicit_ko)
  explicit_ko = kgdb_common.trim(explicit_ko or "")
  if explicit_ko ~= "" then
    if kgdb_common.file_exists(explicit_ko) then
      return explicit_ko
    end
    gdbforge.print("WARN: GDBFORGE_KGDB_KO not found: " .. explicit_ko)
  end
  module_name = kgdb_common.trim(module_name or "")
  modules_dir = kgdb_common.trim(modules_dir or "")
  if module_name == "" or modules_dir == "" then
    return ""
  end
  local ko_name = module_name .. ".ko"
  local try_paths = {
    modules_dir .. "/extra/" .. module_name .. "/" .. ko_name,
    modules_dir .. "/drivers/hwmon/" .. ko_name,
    modules_dir .. "/" .. ko_name,
  }
  for _, path in ipairs(try_paths) do
    if kgdb_common.file_exists(path) then
      return path
    end
  end
  local code, out = kgdb_common.run_cmd(string.format(
    "find %s -name %s -type f 2>/dev/null",
    kgdb_common.shell_quote(modules_dir),
    kgdb_common.shell_quote(ko_name)
  ))
  local best, best_score = "", 0
  for line in tostring(out or ""):gmatch("[^\r\n]+") do
    line = kgdb_common.trim(line)
    if line ~= "" and code == 0 then
      local score = 2
      if line:find("/deploy/lib/modules/", 1, true) then
        score = 1
      elseif line:find("/drivers/", 1, true) or line:find("/extra/", 1, true) then
        score = 3
      end
      if score > best_score then
        best, best_score = line, score
      end
    end
  end
  return best
end

--- True when /sys/module/<name> exists on the target (module loaded).
function kgdb_common.module_loaded(user, host, module_name)
  module_name = kgdb_common.trim(module_name or "")
  if module_name == "" then
    return false
  end
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  local code, out = kgdb_common.ssh_run(user, host, string.format(
    "test -r /sys/module/%s/sections/.text && echo __KGDB_OK__", module_name
  ))
  return code == 0 and tostring(out):find("__KGDB_OK__", 1, true) ~= nil
end

--- modprobe on target if module sections are missing (needs network SSH while in kgdb).
function kgdb_common.ensure_module_loaded(user, host, module_name, do_modprobe)
  module_name = kgdb_common.trim(module_name or "")
  if module_name == "" then
    return false, "empty module name"
  end
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  if kgdb_common.module_loaded(user, host, module_name) then
    gdbforge.print("module " .. module_name .. " present on target (/sys/module/…/sections)")
    return true, nil
  end
  if do_modprobe == false then
    return false, "module " .. module_name .. " not loaded (GDBFORGE_KGDB_MODPROBE=0)"
  end
  gdbforge.print("module " .. module_name .. " missing — ssh modprobe …")
  local _, out = kgdb_common.ssh_run(user, host, string.format(
    "modprobe %s 2>&1; test -r /sys/module/%s/sections/.text && echo __KGDB_OK__",
    module_name, module_name
  ))
  if tostring(out):find("__KGDB_OK__", 1, true) then
    gdbforge.print("module " .. module_name .. " loaded via modprobe")
    return true, nil
  end
  local detail = kgdb_common.trim(out or "")
  if detail == "" then
    detail = "modprobe " .. module_name ..
      " failed (kernel in kgdb? resume, modprobe, re-enter kgdb)"
  end
  return false, detail
end

local function ssh_sysfs_all_sections(user, host, module_name)
  user = kgdb_common.trim(user or kgdb_common.kgdb_user())
  host = kgdb_common.trim(host or kgdb_common.kgdb_host())
  module_name = kgdb_common.trim(module_name or "")

  -- Step 1: list dotfile section names (ls -1A, not plain ls).
  local code, listing = kgdb_common.ssh_run(user, host, string.format(
    "test -d /sys/module/%s/sections || exit 2; ls -1A /sys/module/%s/sections 2>/dev/null",
    module_name, module_name))
  listing = kgdb_common.trim(listing or "")
  if code ~= 0 or listing == "" then
    local hint = listing
    if hint == "" then
      hint = "no /sys/module/" .. module_name ..
        "/sections — modprobe " .. module_name .. " while kernel is running"
    end
    return nil, hint
  end

  -- Step 2: cat each section (avoid for-$(ls) glob issues over ssh/busybox).
  local names = {}
  for line in listing:gmatch("[^\r\n]+") do
    line = kgdb_common.trim(line)
    if line ~= "" and line:match("^[%w%.%-]+$") then
      names[#names + 1] = line
    end
  end
  if #names == 0 then
    return nil, "no section names in listing: " .. listing
  end

  local cat_cmds = {}
  for _, name in ipairs(names) do
    cat_cmds[#cat_cmds + 1] = string.format(
      "printf '%%s=%%s\\n' '%s' \"$(cat /sys/module/%s/sections/%s 2>/dev/null)\"",
      name, module_name, name)
  end
  local code2, out = kgdb_common.ssh_run(user, host, table.concat(cat_cmds, "; "))
  if code2 ~= 0 then
    return nil, kgdb_common.trim(out or "") ~= "" and out or "ssh cat sections failed"
  end

  local sections = {}
  for line in tostring(out):gmatch("[^\r\n]+") do
    local name, addr = line:match("^(.-)=(.+)$")
    name = kgdb_common.trim(name or "")
    addr = kgdb_common.trim(addr or "")
    if name ~= "" and addr ~= "" then
      sections[name] = addr
    end
  end
  if not sections[".text"] then
    local detail = kgdb_common.trim(out or "")
    if detail == "" then
      detail = "read .text failed — is " .. module_name .. " loaded on target?"
    end
    return nil, detail
  end
  return sections, out
end

local function section_for_add_symbol(name)
  if name == ".text" then
    return true
  end
  if name:sub(1, 6) == ".note." or name == ".symtab" or name == ".strtab" then
    return false
  end
  return true
end

local function build_add_symbol_file_cmd(ko, sections)
  local text = sections[".text"]
  local parts = { "add-symbol-file", ko, text }
  local names = {}
  for name in pairs(sections) do
    if name ~= ".text" and section_for_add_symbol(name) then
      names[#names + 1] = name
    end
  end
  table.sort(names)
  for _, name in ipairs(names) do
    parts[#parts + 1] = "-s"
    parts[#parts + 1] = name
    parts[#parts + 1] = sections[name]
  end
  return table.concat(parts, " ")
end

--- add-symbol-file for one loaded module (.ko + sysfs sections). Returns ok, detail.
function kgdb_common.load_module_add_symbol(opts)
  opts = opts or {}
  local mod = kgdb_common.trim(opts.module_name or "")
  local user = kgdb_common.trim(opts.ssh_user or "")
  local host = kgdb_common.trim(opts.ssh_host or "")
  if mod == "" then
    return false, "no module_name"
  end

  local do_modprobe = opts.modprobe
  if do_modprobe == nil then
    do_modprobe = true
  end
  local ok, err = kgdb_common.ensure_module_loaded(user, host, mod, do_modprobe)
  if not ok then
    return false, err
  end

  local ko = kgdb_common.trim(opts.ko_path or "")
  if ko == "" then
    ko = kgdb_common.resolve_module_ko(opts.modules_dir, mod, "")
  end
  if ko == "" or not kgdb_common.file_exists(ko) then
    return false, "missing .ko for " .. mod .. " — set GDBFORGE_KGDB_KO"
  end
  gdbforge.print("using " .. ko)

  gdbforge.print("reading /sys/module/" .. mod .. "/sections via ssh …")
  local sections, serr = ssh_sysfs_all_sections(user, host, mod)
  if not sections then
    local detail = kgdb_common.trim(serr or "")
    if detail == "" then
      detail = "no sysfs sections — modprobe " .. mod .. " while kernel runs, then retry"
    end
    return false, detail
  end
  gdbforge.print("  .text=" .. sections[".text"] ..
    (sections[".data"] and (" .data=" .. sections[".data"]) or ""))

  local cmd = build_add_symbol_file_cmd(ko, sections)
  gdbforge.print(cmd)
  ok, err = kgdb_common.gdb_query(cmd, opts.timeout or 60)
  if not ok then
    return false, "add-symbol-file failed: " .. tostring(err)
  end

  local verify = kgdb_common.trim(opts.verify_pattern or "")
  if verify ~= "" then
    local vok, vout = kgdb_common.gdb_query("info functions " .. verify, 15)
    if vok and tostring(vout):find(verify, 1, true) then
      gdbforge.print("symbols OK — info functions " .. verify .. ":")
      for line in tostring(vout):gmatch("[^\r\n]+") do
        line = kgdb_common.trim(line)
        if line ~= "" and line:find(verify, 1, true) then
          gdbforge.print("  " .. line)
        end
      end
    else
      gdbforge.print("WARN: add-symbol-file ran but 'info functions " .. verify .. "' empty")
      gdbforge.print("  set GDBFORGE_KGDB_VERIFY= or use break file:line in :b gdb")
    end
  end
  return true, nil
end

--- Load module / kernel symbols after target remote.
-- opts:
--   modules_dir  — passed to lx-symbols (optional)
--   skip_lx      — if true, skip lx-symbols
--   ko_path      — local .ko for add-symbol-file
--   text, data, bss — addresses (strings) for add-symbol-file
--   uart_only      — if true, skip SSH module load (kgdb_uart)
--   ssh_user, ssh_host, module_name — SSH sysfs → add-symbol-file (kgdb_kdmx / kgdb_load_module)
--   scripts, vmlinux — locate kernel scripts/gdb for lx-symbols
function kgdb_common.load_symbols(opts)
  opts = opts or {}
  local ok_all = true

  local mod = kgdb_common.trim(opts.module_name or "")
  if mod ~= "" and not opts.uart_only then
    local ok, detail = kgdb_common.load_module_add_symbol(opts)
    if not ok then
      ok_all = false
      gdbforge.print("ERROR: module symbols — " .. tostring(detail))
      gdbforge.print("  ensure module is loaded on target (ssh: modprobe " .. mod .. ")")
      gdbforge.print("  retry: :lua kgdb_load_module " .. mod)
    end
  end

  if opts.skip_lx then
    return ok_all
  end

  local has_scripts = kgdb_common.source_kernel_scripts(opts.scripts, opts.vmlinux, opts.modules_dir)
  if has_scripts then
    local mods = kgdb_common.trim(opts.modules_dir or "")
    if mods ~= "" then
      gdbforge.print("lx-symbols " .. mods)
      gdbforge.gdb("lx-symbols " .. mods)
    else
      gdbforge.print("lx-symbols")
      gdbforge.gdb("lx-symbols")
    end
  end
  return ok_all
end

function help()
  gdbforge.print("kgdb_common — shared helpers for kgdb_* scripts (not a workflow)")
  gdbforge.print("Use: :lua kgdb_kdmx  :lua kgdb_load_module  :lua kgdb_detach  :lua kgdb_trigger")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main()
  help()
end

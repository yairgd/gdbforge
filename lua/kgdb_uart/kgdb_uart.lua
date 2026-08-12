-- kgdb over shared UART via external kdmx — one-shot → stopped debug mode.
-- Install: cp -r lua/kgdb_uart lua/kgdb_common .gdbforge/lua/
-- Docs:    docs/KERNEL_KGDB.md
--
-- Usage:   :lua kgdb_uart [module]
-- Example: :lua kgdb_uart 8250
--
-- Env:
--   GDBFORGE_KGDB_UART        serial device (required), e.g. /dev/ttyUSB0
--   GDBFORGE_KGDB_BAUD        default 115200
--   GDBFORGE_KGDB_VMLINUX     path to vmlinux
--   GDBFORGE_KGDB_MODULES     optional dir for lx-symbols
--   GDBFORGE_KGDB_SCRIPTS     kernel tree holding scripts/gdb (for lx-symbols)
--   GDBFORGE_KGDB_KDMX        kdmx binary (default kdmx)
--   GDBFORGE_KGDB_CONSOLE_CMD optional: program for console PTY (default minicom)
--   GDBFORGE_KGDB_TAKEOVER    how to claim the UART for kdmx:
--                               auto  (default) kill known serial consoles
--                               force          kill any holder
--                               never          abort while the UART is held
--   GDBFORGE_KGDB_FORCE       deprecated alias for GDBFORGE_KGDB_TAKEOVER=force
--   GDBFORGE_KGDB_RETRIES     kdmx start attempts (default 3)
--   GDBFORGE_KGDB_SYMBOL_WAIT seconds to let "file vmlinux" finish (default 5)
--   GDBFORGE_TERMINAL         emulator for spawn_terminal
--
-- kdmx must own the UART exclusively; a minicom already attached to the raw
-- device is terminated first, then reopened on the console PTY.
--
-- Board must be waiting in kgdb (kgdboc=…,kgdbwait or already broken in).

-- kgdb_common lives in a sibling directory, which moves with the install layer
-- (project, home, embedded cache). Search every layer so a partial install or a
-- stale embedded cache still resolves.
local function common_candidates()
  local rel = "/kgdb_common/kgdb_common.lua"
  local list = { gdbforge.lua_dir() .. "/.." .. rel, "./.gdbforge/lua" .. rel }
  local home = os.getenv("HOME")
  if home and home ~= "" then
    list[#list + 1] = home .. "/.gdbforge/lua" .. rel
    list[#list + 1] = home .. "/.cache/gdbforge/embedded-lua" .. rel
  end
  return list
end

local function load_common()
  local tried = {}
  for _, path in ipairs(common_candidates()) do
    local fh = io.open(path, "r")
    if fh then
      fh:close()
      local ok, err = pcall(function()
        dofile(path)
      end)
      if ok then
        return true
      end
      gdbforge.print("ERROR: cannot load kgdb_common from " .. path)
      gdbforge.print(tostring(err))
      return false
    end
    tried[#tried + 1] = path
  end

  gdbforge.print("ERROR: kgdb_common.lua not found. Looked in:")
  for _, path in ipairs(tried) do
    gdbforge.print("  " .. path)
  end
  gdbforge.print("Install: cp -r lua/kgdb_common lua/kgdb_uart .gdbforge/lua/")
  return false
end

function help()
  gdbforge.print("kgdb_uart — kdmx + minicom + target remote → stopped debug mode")
  gdbforge.print("Usage: :lua kgdb_uart [module]")
  gdbforge.print("  :lua kgdb_uart 8250")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_KGDB_UART=/dev/ttyUSB0")
  gdbforge.print("  export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux")
  gdbforge.print("  export GDBFORGE_KGDB_MODULES=/path/to/kernel/build   # optional")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("  # board: kgdboc=<uart>,kgdbwait  (kernel waiting in kgdb)")
  gdbforge.print("  # host:  kdmx on PATH (agent-proxy)")
  gdbforge.print("The UART is taken over by kdmx — any minicom on the raw device is closed")
  gdbforge.print("and reopened on the console PTY. Control this with:")
  gdbforge.print("  export GDBFORGE_KGDB_TAKEOVER=auto|force|never   # default auto")
  gdbforge.print("After attach (stopped): set BP, continue, trigger in minicom")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main(module_name)
  if not load_common() then
    return
  end
  local C = kgdb_common

  local uart = C.env("GDBFORGE_KGDB_UART", "")
  if uart == "" then
    gdbforge.print("ERROR: set GDBFORGE_KGDB_UART=/dev/ttyUSB0")
    return
  end
  local baud = C.env("GDBFORGE_KGDB_BAUD", "115200")
  local vmlinux = C.env("GDBFORGE_KGDB_VMLINUX", "")
  local modules = C.env("GDBFORGE_KGDB_MODULES", "")
  local scripts = C.env("GDBFORGE_KGDB_SCRIPTS", "")
  local kdmx = C.resolve_kdmx(C.env("GDBFORGE_KGDB_KDMX", ""), gdbforge.lua_dir())
  local console_cmd = C.env("GDBFORGE_KGDB_CONSOLE_CMD", "minicom")
  local takeover = C.env("GDBFORGE_KGDB_TAKEOVER", "auto"):lower()
  local legacy_force = C.env("GDBFORGE_KGDB_FORCE", "0"):lower()
  if legacy_force == "1" or legacy_force == "true" then
    takeover = "force"
  end
  local retries = tonumber(C.env("GDBFORGE_KGDB_RETRIES", "3")) or 3
  module_name = C.trim(module_name)

  if takeover ~= "auto" and takeover ~= "force" and takeover ~= "never" then
    gdbforge.print("ERROR: GDBFORGE_KGDB_TAKEOVER must be auto, force or never")
    return
  end

  if not C.file_exists(uart) then
    gdbforge.print("ERROR: no such device: " .. uart)
    return
  end

  local kdmx_ok, kdmx_ver = C.check_kdmx(kdmx)
  gdbforge.print("kdmx: " .. kdmx .. "  (" .. kdmx_ver .. ")")
  if not kdmx_ok then
    gdbforge.print("WARN: this kdmx exits when a serial read returns EAGAIN, which")
    gdbforge.print("      closes both PTYs — GDB then reports 'Remote connection closed'.")
    gdbforge.print("      Build the patched one, or point at it explicitly:")
    gdbforge.print("        export GDBFORGE_KGDB_KDMX=/path/to/gdbforge/bin/kdmx")
  end

  gdbforge.print("claiming " .. uart .. " for kdmx (takeover=" .. takeover .. ") …")
  if not C.free_device(uart, takeover) then
    return
  end

  C.run_cmd("rm -f /tmp/gdbforge-kdmx-*_gdb /tmp/gdbforge-kdmx-*_trm 2>/dev/null")

  local log = "/tmp/gdbforge-kdmx.log"
  local gdb_pty, trm_pty, pid

  for attempt = 1, math.max(1, retries) do
    local status = string.format("/tmp/gdbforge-kdmx-%d-%d", os.time(), attempt)
    local status_gdb, status_trm = status .. "_gdb", status .. "_trm"

    local start = string.format(
      "nohup %s -p %s -b %s -s %s >%s 2>&1 </dev/null & echo $!",
      C.shell_quote(kdmx), C.shell_quote(uart), C.shell_quote(baud),
      C.shell_quote(status), C.shell_quote(log)
    )
    gdbforge.print(string.format("starting kdmx on %s @%s (attempt %d/%d) …",
      uart, baud, attempt, math.max(1, retries)))
    local code, out = C.run_cmd(start)
    if code ~= 0 then
      gdbforge.print("ERROR: failed to start kdmx: " .. C.trim(out))
      return
    end
    pid = C.trim(out):match("(%d+)")

    local g = C.wait_file(status_gdb, 15)
    local t = C.wait_file(status_trm, 15)
    if g and t and C.pid_alive(pid) then
      -- kdmx can still abort right after publishing the PTYs (e.g. another
      -- reader on the UART), which surfaces later as "Remote connection closed".
      gdbforge.sleep(1)
      if C.pid_alive(pid) then
        gdb_pty = g:match("(/dev/pts/%d+)") or g
        trm_pty = t:match("(/dev/pts/%d+)") or t
        break
      end
    end

    gdbforge.print("kdmx did not come up — log:")
    local _, tail = C.run_cmd("tail -5 " .. C.shell_quote(log) .. " 2>/dev/null")
    gdbforge.print(C.trim(tail))
    if pid then
      C.run_cmd("kill -9 " .. pid .. " 2>/dev/null")
    end
    gdbforge.sleep(1)
  end

  if not gdb_pty or not trm_pty then
    gdbforge.print("ERROR: kdmx failed to serve " .. uart .. " — check " .. log)
    gdbforge.print("  'unexpected errno 11' usually means another program is reading the UART")
    return
  end

  gdbforge.print("kdmx pid " .. tostring(pid) .. "  (log " .. log .. ")")
  gdbforge.print("console PTY: " .. trm_pty)
  gdbforge.print("gdb PTY:     " .. gdb_pty)

  if console_cmd == "minicom" then
    gdbforge.print("opening minicom on console PTY …")
    gdbforge.spawn_terminal("minicom", "-D", trm_pty, "-o")
  else
    gdbforge.print("opening " .. console_cmd .. " on console PTY …")
    gdbforge.spawn_terminal(console_cmd, trm_pty)
  end

  gdbforge.open_buffer("gdb")
  if vmlinux ~= "" then
    gdbforge.print("file " .. vmlinux)
    gdbforge.gdb("file " .. vmlinux)
    -- vmlinux is large; the kernel GDB scripts abort with "No symbol table is
    -- loaded" if they are sourced before this finishes.
    gdbforge.sleep(tonumber(C.env("GDBFORGE_KGDB_SYMBOL_WAIT", "5")) or 5)
  else
    gdbforge.print("WARN: GDBFORGE_KGDB_VMLINUX unset — using current GDB symbols")
  end

  gdbforge.print("target remote " .. gdb_pty)
  gdbforge.print("(board must be waiting in kgdb — attach may block until then)")
  gdbforge.gdb("target remote " .. gdb_pty)
  gdbforge.sleep(2)

  if not C.pid_alive(pid) then
    gdbforge.print("ERROR: kdmx exited during attach — check " .. log)
    return
  end

  C.load_symbols({
    modules_dir = modules,
    module_name = module_name,
    scripts = scripts,
    vmlinux = vmlinux,
  })

  if module_name ~= "" then
    gdbforge.print("module hint: " .. module_name .. " (set BP, then continue; trigger from minicom)")
  end
  gdbforge.print("kgdb_uart: STOPPED in debug mode — set breakpoints, then continue")
end

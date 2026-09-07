-- One-shot kernel kgdb over UART: kgdboc + kdmx + minicom + SysRq-G + target remote.
-- Install: cp -r lua/kgdb_kdmx lua/kgdb_common .gdbforge/lua/
-- Docs:    docs/KERNEL_KGDB.md
--
-- Usage:   :lua kgdb_kdmx
--          :lua kgdb_kdmx [module]
--          :lua kgdb_kdmx symbols
--          :lua kgdb_kdmx help
--
-- All settings via env (defaults match typical board setup):
--   GDBFORGE_KGDB_UART=/dev/ttyUSB0
--   GDBFORGE_KGDB_BOARD_TTY=ttyPS0
--   GDBFORGE_KGDB_HOST=192.168.20.50
--   GDBFORGE_KGDB_SSH_USER=root
--   GDBFORGE_KGDB_BAUD=115200
--   GDBFORGE_KGDB_MODULE=mydriver          optional
--   GDBFORGE_KGDB_KO=/path/to/mydriver.ko  optional
--   GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux
--   GDBFORGE_KGDB_MODULES=/path/to/kernel-source
--   GDBFORGE_KGDB_SCRIPTS=/path/to/kernel-source
--   GDBFORGE_KGDB_SETUP=1          SSH kgdboc (step 1), default on
--   GDBFORGE_KGDB_SYSRQ=1          SSH sysrq-g during target remote (step 5)
--   GDBFORGE_KGDB_SYSRQ_WAIT=1     Seconds before sysrq after target remote starts
--   GDBFORGE_KGDB_KDMX=kdmx
--   GDBFORGE_KGDB_TAKEOVER=auto
--   GDBFORGE_KGDB_SYMBOL_WAIT=120  gdb_query timeout for file vmlinux (seconds)
--   GDBFORGE_KGDB_ATTACH_WAIT=120    gdb_query timeout for target remote
--   GDBFORGE_TERMINAL=mate-terminal
--   GDBFORGE_KGDB_CLEANUP=1       auto cleanup before start (default on)
--   GDBFORGE_SSH_OPTS=-o BatchMode=yes -o StrictHostKeyChecking=accept-new
--
--   GDBFORGE_KGDB_VERIFY=           optional info functions pattern after add-symbol-file
--
-- Manual cleanup only: :lua kgdb_detach

local defaults = {
  uart = "/dev/ttyUSB0",
  board_tty = "ttyPS0",
  host = "192.168.20.50",
  user = "root",
  baud = "115200",
  module = "",
  verify_pattern = "",
  kernel_tree = "",
  vmlinux = "",
}

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
  for _, path in ipairs(common_candidates()) do
    local fh = io.open(path, "r")
    if fh then
      fh:close()
      local ok, err = pcall(function() dofile(path) end)
      if ok then return true end
      gdbforge.print("ERROR: cannot load kgdb_common: " .. tostring(err))
      return false
    end
  end
  gdbforge.print("ERROR: kgdb_common.lua not found — cp -r lua/kgdb_common lua/kgdb_kdmx .gdbforge/lua/")
  return false
end

local function env_bool(name, default_on)
  local v = os.getenv(name)
  if v == nil or kgdb_common.trim(v) == "" then
    return default_on
  end
  v = kgdb_common.trim(v):lower()
  return v == "1" or v == "true" or v == "yes" or v == "on"
end

function help()
  gdbforge.print("kgdb_kdmx — one script: kgdboc + kdmx + minicom + sysrq + GDB")
  gdbforge.print("Usage:")
  gdbforge.print("  :lua kgdb_kdmx")
  gdbforge.print("  :lua kgdb_kdmx [module]")
  gdbforge.print("  :lua kgdb_load_module     reload module symbols (already attached)")
  gdbforge.print("  :lua kgdb_load_module mydriver")
  gdbforge.print("After attach: set breakpoints in :b gdb yourself, then continue")
  gdbforge.print("Defaults (override with export):")
  gdbforge.print("  GDBFORGE_KGDB_UART=" .. defaults.uart)
  gdbforge.print("  GDBFORGE_KGDB_BOARD_TTY=" .. defaults.board_tty)
  gdbforge.print("  GDBFORGE_KGDB_HOST=" .. defaults.host)
  gdbforge.print("  GDBFORGE_KGDB_SSH_USER=" .. defaults.user)
  gdbforge.print("  GDBFORGE_KGDB_BAUD=" .. defaults.baud)
  gdbforge.print("  GDBFORGE_KGDB_MODULE=     optional driver module name")
  gdbforge.print("  GDBFORGE_KGDB_KO=         optional path to module.ko")
  gdbforge.print("  GDBFORGE_KGDB_VMLINUX=    path to vmlinux")
  gdbforge.print("  GDBFORGE_KGDB_MODULES=    kernel build tree")
  gdbforge.print("  GDBFORGE_KGDB_VERIFY=     optional info functions check after add-symbol-file")
  gdbforge.print("  GDBFORGE_KGDB_CLEANUP=1   (auto before start; :lua kgdb_detach)")
  gdbforge.print("  GDBFORGE_KGDB_SYSRQ=1      sysrq during target remote (step 5)")
  gdbforge.print("  GDBFORGE_KGDB_SYSRQ_WAIT=1 delay before sysrq (seconds)")
  gdbforge.print("  GDBFORGE_KGDB_MODE=1      CLI n/s (auto via :lua kgdb_kdmx)")
  gdbforge.print("Re-enter kgdb: :lua kgdb_trigger")
end

local function module_symbol_opts(C, module_name, user, host, modules, scripts, vmlinux)
  return {
    modules_dir = modules,
    scripts = scripts,
    vmlinux = vmlinux,
    module_name = module_name,
    ko_path = C.resolve_module_ko(modules, module_name,
      C.env("GDBFORGE_KGDB_KO", "")),
    ssh_user = user,
    ssh_host = host,
    verify_pattern = C.env("GDBFORGE_KGDB_VERIFY", defaults.verify_pattern),
    skip_lx = true,
  }
end

function main(arg)
  if not load_common() then return end
  local C = kgdb_common

  arg = C.trim(arg or "")
  if arg == "help" then
    help()
    return
  end

  local module_name = C.env("GDBFORGE_KGDB_MODULE", defaults.module)
  if arg == "symbols" or arg == "reload" then
    gdbforge.print("kgdb_kdmx: use :lua kgdb_load_module" ..
      (module_name ~= "" and (" " .. module_name) or ""))
    return
  end
  if arg ~= "" then
    module_name = arg
  end

  local uart = C.env("GDBFORGE_KGDB_UART", defaults.uart)
  local board_tty = C.resolve_board_tty(defaults.board_tty)
  local host = C.kgdb_host()
  local user = C.kgdb_user()
  local baud = C.env("GDBFORGE_KGDB_BAUD", defaults.baud)
  local vmlinux = C.env("GDBFORGE_KGDB_VMLINUX", defaults.vmlinux)
  local modules = C.env("GDBFORGE_KGDB_MODULES", defaults.kernel_tree)
  local scripts = C.env("GDBFORGE_KGDB_SCRIPTS", defaults.kernel_tree)
  local kdmx = C.resolve_kdmx(C.env("GDBFORGE_KGDB_KDMX", ""), gdbforge.lua_dir())
  local takeover = C.env("GDBFORGE_KGDB_TAKEOVER", "auto"):lower()
  local status = C.kdmx_status_prefix
  local do_setup = env_bool("GDBFORGE_KGDB_SETUP", true)
  local do_sysrq = env_bool("GDBFORGE_KGDB_SYSRQ", true)
  local do_cleanup = env_bool("GDBFORGE_KGDB_CLEANUP", true)

  gdbforge.print("kgdb_kdmx: uart=" .. uart .. " board=" .. board_tty ..
    " ssh=" .. C.kgdb_ssh_target(user, host) .. " baud=" .. baud)

  gdbforge.print("step 0: kill stale kdmx on " .. uart)
  C.kill_kdmx(uart)

  if gdbforge.set_kgdb_mode then
    gdbforge.set_kgdb_mode(true)
    gdbforge.print("kgdb mode: CLI next/step (like manual cgdb), lighter stop queries")
  end

  if do_cleanup then
    C.kgdb_cleanup({
      user = user,
      host = host,
      uart = uart,
      baud = baud,
      kdmx = kdmx,
    })
  end

  if do_setup then
    gdbforge.print("step 1: echo " .. board_tty .. "," .. baud .. " > kgdboc (ssh)")
    local ok, detail = C.setup_kgdboc(user, host, board_tty, baud)
    if not ok then
      gdbforge.print("ERROR: kgdboc — " .. tostring(detail))
      return
    end
  else
    gdbforge.print("step 1: skipped (GDBFORGE_KGDB_SETUP=0)")
  end

  gdbforge.print("step 2: kdmx -n -p " .. uart .. " -b " .. baud .. " -s " .. status)
  local gdb_pty, trm_pty, pid, log = C.start_kdmx({
    uart = uart,
    baud = baud,
    kdmx = kdmx,
    takeover = takeover,
    status_prefix = status,
  })
  if not gdb_pty then
    gdbforge.print("ERROR: " .. tostring(trm_pty))
    return
  end

  gdbforge.print("step 3: minicom -D $(cat " .. status .. "_trm)")
  C.open_serial_console(trm_pty, "minicom")

  gdbforge.open_buffer("gdb")
  if vmlinux ~= "" then
    local sym_wait = tonumber(C.env("GDBFORGE_KGDB_SYMBOL_WAIT", "120")) or 120
    gdbforge.print("step 4a: file " .. vmlinux .. " (wait up to " .. sym_wait .. "s)")
    local ok, detail = C.gdb_query("file " .. vmlinux, sym_wait)
    if not ok then
      gdbforge.print("ERROR: file vmlinux — " .. tostring(detail))
      return
    end
  else
    gdbforge.print("WARN: vmlinux not set — export GDBFORGE_KGDB_VMLINUX")
  end

  gdb_pty = C.read_file(status .. "_gdb") or gdb_pty

  gdbforge.print("step 4b: GDB tune (remotetimeout; baud " .. baud .. " is kdmx/kgdboc)")
  C.kgdb_remote_tune(baud)
  gdbforge.sleep(0.5)

  local attach_wait = tonumber(C.env("GDBFORGE_KGDB_ATTACH_WAIT", "120")) or 120
  local sysrq_delay = tonumber(C.env("GDBFORGE_KGDB_SYSRQ_WAIT", "1")) or 1

  if module_name ~= "" then
    gdbforge.print("step 4c: modprobe " .. module_name .. " on target (before kgdb break)")
    local ok, detail = C.ensure_module_loaded(user, host, module_name, true)
    if not ok then
      gdbforge.print("WARN: module preload — " .. tostring(detail))
      gdbforge.print("  step 6 will retry after attach; or: ssh modprobe " .. module_name)
    end
  end

  if do_sysrq then
    gdbforge.print("step 5: GDB target remote + SSH sysrq-g (parallel)")
    local ok, detail = C.gdb_attach_with_sysrq(gdb_pty, {
      user = user,
      host = host,
      attach_wait = attach_wait,
      sysrq_delay = sysrq_delay,
      sysrq = true,
    })
    if not ok then
      gdbforge.print("ERROR: attach — " .. tostring(detail))
      gdbforge.print("  at [1]kdb> in minicom you can type: kgdb")
      gdbforge.print("  then in :b gdb: target remote " .. gdb_pty)
      return
    end
  else
    gdbforge.print("step 5: target remote " .. gdb_pty .. " (GDBFORGE_KGDB_SYSRQ=0)")
    local ok, detail = C.gdb_target_remote(gdb_pty, attach_wait)
    if not ok then
      gdbforge.print("ERROR: target remote — " .. tostring(detail))
      return
    end
  end
  gdbforge.sleep(0.5)

  if not C.pid_alive(pid) then
    gdbforge.print("ERROR: kdmx exited — check " .. tostring(log))
    return
  end

  if module_name ~= "" then
    gdbforge.print("step 6: load module " .. module_name .. " (add-symbol-file)")
    local sym_ok = C.load_symbols(module_symbol_opts(C, module_name, user, host, modules, scripts, vmlinux))
    if not sym_ok then
      gdbforge.print("WARN: module symbols not loaded — breakpoints on module code will stay pending")
      gdbforge.print("  retry: :lua kgdb_load_module " .. module_name)
    end
  else
    gdbforge.print("step 6: skipped (no module — :lua kgdb_kdmx <name> or GDBFORGE_KGDB_MODULE)")
  end

  gdbforge.print("kgdb_kdmx: ready — set breakpoints in :b gdb, then continue")
  if do_sysrq then
    gdbforge.print("  re-enter while running: :lua kgdb_trigger  (not GDB Ctrl+C)")
  else
    gdbforge.print("  trigger kgdb: echo g in minicom or :lua kgdb_trigger, then target remote")
  end
end

-- kgdb over Ethernet / separate console — one-shot → stopped debug mode.
-- No kdmx (console is SSH or another UART; GDB uses TCP).
-- Install: cp -r lua/kernel/kgdb_net lua/kgdb_common .gdbforge/lua/
-- Docs:    docs/KERNEL_KGDB.md
--
-- Usage:   :lua kgdb_net [module] [host] [port]
--
-- Env:
--   GDBFORGE_KGDB_VMLINUX     path to vmlinux
--   GDBFORGE_KGDB_MODULES     optional dir for lx-symbols
--   GDBFORGE_KGDB_PORT        kgdb TCP port (default 6443)
--   GDBFORGE_REMOTE_HOST      board IP (default 192.168.20.50)
--   GDBFORGE_REMOTE_USER      SSH user for optional console / sysfs (default root)
--   GDBFORGE_KGDB_KO          optional local .ko for add-symbol-file via SSH sysfs
--   GDBFORGE_KGDB_SSH_CONSOLE if "1", open ssh -t in external terminal
--   GDBFORGE_TERMINAL         emulator for spawn_terminal
--
-- Board kgdb stub must listen (e.g. kgdboe) before / while attaching.

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
    list[#list + 1] = home .. "/.cache/gdbforge/embedded-lua/kernel" .. rel
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
  gdbforge.print("Install: cp -r lua/kernel/kgdb_common lua/kgdb_net .gdbforge/lua/")
  return false
end

function help()
  gdbforge.print("kgdb_net — target remote host:port → stopped debug mode (no kdmx)")
  gdbforge.print("Usage: :lua kgdb_net [module] [host] [port]")
  gdbforge.print("Setup:")
  gdbforge.print("  export GDBFORGE_REMOTE_HOST=192.168.20.50")
  gdbforge.print("  export GDBFORGE_KGDB_PORT=6443")
  gdbforge.print("  export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux")
  gdbforge.print("  export GDBFORGE_KGDB_MODULES=/path/to/kernel/build   # optional")
  gdbforge.print("  export GDBFORGE_KGDB_SSH_CONSOLE=1   # optional SSH shell window")
  gdbforge.print("After attach (stopped): set BP, continue, trigger from console")
  gdbforge.print("Docs: docs/KERNEL_KGDB.md")
end

function main(module_name, host, port)
  if not load_common() then
    return
  end
  local C = kgdb_common

  module_name = C.trim(module_name)
  host = C.trim(host)
  if host == "" then
    host = C.env("GDBFORGE_REMOTE_HOST", "192.168.20.50")
  end
  port = C.trim(port)
  if port == "" then
    port = C.env("GDBFORGE_KGDB_PORT", "6443")
  end
  local user = C.env("GDBFORGE_REMOTE_USER", "root")
  local vmlinux = C.env("GDBFORGE_KGDB_VMLINUX", "")
  local modules = C.env("GDBFORGE_KGDB_MODULES", "")
  local ko = C.env("GDBFORGE_KGDB_KO", "")
  local ssh_console = C.env("GDBFORGE_KGDB_SSH_CONSOLE", "")
  local addr = host .. ":" .. port

  if ssh_console == "1" or ssh_console:lower() == "true" then
    gdbforge.print("opening SSH console …")
    gdbforge.spawn_terminal("ssh", "-t", user .. "@" .. host)
  end

  gdbforge.open_buffer("gdb")
  if vmlinux ~= "" then
    gdbforge.print("file " .. vmlinux)
    gdbforge.gdb("file " .. vmlinux)
  else
    gdbforge.print("WARN: GDBFORGE_KGDB_VMLINUX unset — using current GDB symbols")
  end

  gdbforge.print("waiting for " .. addr .. " …")
  if not gdbforge.wait_port(addr, 30) then
    gdbforge.print("ERROR: nothing listening on " .. addr)
    gdbforge.print("Ensure kgdboe (or other TCP kgdb stub) is up, or board is waiting")
    return
  end

  gdbforge.print("target remote " .. addr)
  gdbforge.gdb("target remote " .. addr)

  C.load_symbols({
    modules_dir = modules,
    module_name = module_name,
    ko_path = ko,
    ssh_user = user,
    ssh_host = host,
    scripts = C.env("GDBFORGE_KGDB_SCRIPTS", ""),
    vmlinux = vmlinux,
  })

  if module_name ~= "" then
    gdbforge.print("module hint: " .. module_name .. " (set BP, then continue)")
  end
  gdbforge.print("kgdb_net: STOPPED in debug mode — set breakpoints, then continue")
end

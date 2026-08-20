-- Load one kernel module into GDB: SSH modprobe + sysfs sections + add-symbol-file.
-- Install: cp -r lua/kernel/kgdb_load_module lua/kgdb_common .gdbforge/lua/
--
-- Manual driver symbols (already attached via target remote):
--   :lua kgdb_load_module as6221
--   :lua kgdb_load_module 8250_of
--
-- Env:
--   GDBFORGE_KGDB_MODULES=/path/to/kernel-source   search tree for module.ko
--   GDBFORGE_KGDB_KO=/path/to/as6221.ko            override .ko path
--   GDBFORGE_KGDB_VERIFY=as6221                    optional info functions check
--   GDBFORGE_KGDB_MODPROBE=1                       ssh modprobe if missing (default on)

local defaults = {
  kernel_tree = "/home/yair/merlin/kernel-source",
  vmlinux = "/home/yair/merlin/kernel-source/vmlinux",
}

-- Optional short names (module must exist as modprobe name on target).
local module_aliases = {
  ["8250"] = "8250_of",
}

local function load_common()
  local paths = {
    gdbforge.lua_dir() .. "/../kgdb_common/kgdb_common.lua",
    "./.gdbforge/lua/kgdb_common/kgdb_common.lua",
  }
  local home = os.getenv("HOME")
  if home and home ~= "" then
    paths[#paths + 1] = home .. "/.gdbforge/lua/kgdb_common/kgdb_common.lua"
    paths[#paths + 1] = home .. "/.cache/gdbforge/embedded-lua/kgdb_common/kgdb_common.lua"
    paths[#paths + 1] = home .. "/.cache/gdbforge/embedded-lua/kernel/kgdb_common/kgdb_common.lua"
  end
  for _, path in ipairs(paths) do
    local fh = io.open(path, "r")
    if fh then
      fh:close()
      local ok, err = pcall(function() dofile(path) end)
      if ok then return true end
      gdbforge.print("ERROR: cannot load kgdb_common: " .. tostring(err))
      return false
    end
  end
  gdbforge.print("ERROR: kgdb_common not found — cp -r lua/kernel/kgdb_common lua/kgdb_load_module .gdbforge/lua/")
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

local function resolve_module_name(arg)
  local C = kgdb_common
  arg = C.trim(arg or "")
  if arg == "" then
    arg = C.env("GDBFORGE_KGDB_MODULE", "")
  end
  if arg == "" then
    return ""
  end
  local alias = module_aliases[arg:lower()]
  if alias then
    gdbforge.print("module alias: " .. arg .. " → " .. alias)
    return alias
  end
  return arg
end

function help()
  gdbforge.print("kgdb_load_module — modprobe + add-symbol-file for any driver module")
  gdbforge.print("Usage (GDB must already be attached: target remote …):")
  gdbforge.print("  :lua kgdb_load_module as6221")
  gdbforge.print("  :lua kgdb_load_module 8250_of")
  gdbforge.print("Then in :b gdb: break <function>  or  break path/to/driver.c:line")
  gdbforge.print("Env:")
  gdbforge.print("  GDBFORGE_KGDB_MODULES=" .. defaults.kernel_tree)
  gdbforge.print("  GDBFORGE_KGDB_KO=/path/to/module.ko     (optional override)")
  gdbforge.print("  GDBFORGE_KGDB_VERIFY=pattern            (optional info functions check)")
  gdbforge.print("  GDBFORGE_KGDB_MODPROBE=1                (default: ssh modprobe if needed)")
  gdbforge.print("Example as6221:")
  gdbforge.print("  ssh root@board modprobe as6221          (while kernel running)")
  gdbforge.print("  :lua kgdb_load_module as6221")
  gdbforge.print("  break as6221_read   # or break extra/as6221/as6221.c:NN")
end

function main(arg)
  if not load_common() then return end
  local C = kgdb_common

  arg = C.trim(arg or "")
  if arg == "help" then
    help()
    return
  end

  local module_name = resolve_module_name(arg)
  if module_name == "" then
    gdbforge.print("ERROR: module name required")
    gdbforge.print("  :lua kgdb_load_module as6221")
    gdbforge.print("  :lua kgdb_load_module help")
    return
  end

  local user = C.kgdb_user()
  local host = C.kgdb_host()
  local modules = C.env("GDBFORGE_KGDB_MODULES", defaults.kernel_tree)
  local scripts = C.env("GDBFORGE_KGDB_SCRIPTS", defaults.kernel_tree)
  local vmlinux = C.env("GDBFORGE_KGDB_VMLINUX", defaults.vmlinux)
  local modprobe = env_bool("GDBFORGE_KGDB_MODPROBE", true)
  local ko_override = C.env("GDBFORGE_KGDB_KO", "")

  gdbforge.open_buffer("gdb")
  gdbforge.print("kgdb_load_module: " .. module_name ..
    " (ssh " .. C.kgdb_ssh_target(user, host) .. ")")
  gdbforge.print("  1) ssh modprobe " .. module_name .. " if not loaded")
  gdbforge.print("  2) read /sys/module/" .. module_name .. "/sections")
  gdbforge.print("  3) gdb add-symbol-file …  (see :b gdb)")
  gdbforge.print("  log: :b io")

  local ko = C.resolve_module_ko(modules, module_name, ko_override)
  if ko == "" then
    gdbforge.print("ERROR: no .ko for " .. module_name .. " under " .. modules)
    gdbforge.print("  export GDBFORGE_KGDB_KO=/path/to/" .. module_name .. ".ko")
    return
  end

  local ok, detail = C.load_module_add_symbol({
    modules_dir = modules,
    scripts = scripts,
    vmlinux = vmlinux,
    module_name = module_name,
    ko_path = ko,
    ssh_user = user,
    ssh_host = host,
    verify_pattern = C.env("GDBFORGE_KGDB_VERIFY", ""),
    modprobe = modprobe,
    skip_lx = true,
  })

  if ok then
    gdbforge.print("kgdb_load_module: OK — " .. module_name .. " symbols in GDB")
    gdbforge.print("  break <function>   or   break path/" .. module_name .. ".c:line")
  else
    gdbforge.print("ERROR: kgdb_load_module — " .. tostring(detail))
    gdbforge.print("  modprobe while kernel runs: ssh " .. C.kgdb_ssh_target(user, host) ..
      " modprobe " .. module_name)
    gdbforge.print("  then re-enter kgdb (:lua kgdb_trigger) and retry")
  end
end

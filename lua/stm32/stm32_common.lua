-- Shared helpers for STM32 ST-Link / OpenOCD Lua scripts.
-- Install: cp lua/stm32/stm32_common.lua .gdbforge/lua/

local M = {}

M.PROFILES = { "baremetal", "zephyr", "freertos" }

local PROFILE_ALIASES = {
  bare = "baremetal",
  baremetal = "baremetal",
  none = "baremetal",
  off = "baremetal",
  zephyr = "zephyr",
  zephyr_rtos = "zephyr",
  freertos = "freertos",
  free_rtos = "freertos",
  rtos = "freertos",
}

-- OpenOCD board/target snippets + optional Zephyr board folder name.
-- Add rows here for new Nucleo kits or generic STM32 targets.
M.BOARDS = {
  nucleo_f401re = {
    label = "Nucleo F401RE (STM32F401RE)",
    zephyr = "nucleo_f401re",
    openocd = { board = "board/st_nucleo_f4.cfg" },
  },
  nucleo_f411re = {
    label = "Nucleo F411RE (STM32F411RE)",
    zephyr = "nucleo_f411re",
    openocd = { board = "board/st_nucleo_f4.cfg" },
  },
  nucleo_f429zi = {
    label = "Nucleo F429ZI (STM32F429ZI)",
    zephyr = "nucleo_f429zi",
    openocd = { board = "board/st_nucleo_f4.cfg" },
  },
  nucleo_f446re = {
    label = "Nucleo F446RE (STM32F446RE)",
    zephyr = "nucleo_f446re",
    openocd = { board = "board/st_nucleo_f4.cfg" },
  },
  nucleo_f303re = {
    label = "Nucleo F303RE (STM32F303RE)",
    zephyr = "nucleo_f303re",
    openocd = { board = "board/st_nucleo_f3.cfg" },
  },
  nucleo_f334r8 = {
    label = "Nucleo F334R8 (STM32F334R8)",
    zephyr = "nucleo_f334r8",
    openocd = { board = "board/st_nucleo_f3.cfg" },
  },
  nucleo_f722ze = {
    label = "Nucleo F722ZE (STM32F722ZE)",
    zephyr = "nucleo_f722ze",
    openocd = { board = "board/st_nucleo_f7.cfg" },
  },
  nucleo_f746zg = {
    label = "Nucleo F746ZG (STM32F746ZG)",
    zephyr = "nucleo_f746zg",
    openocd = { board = "board/st_nucleo_f7.cfg" },
  },
  nucleo_f767zi = {
    label = "Nucleo F767ZI (STM32F767ZI)",
    zephyr = "nucleo_f767zi",
    openocd = { board = "board/st_nucleo_f7.cfg" },
  },
  nucleo_g071rb = {
    label = "Nucleo G071RB (STM32G071RB)",
    zephyr = "nucleo_g071rb",
    openocd = { board = "board/st_nucleo_g0.cfg" },
  },
  nucleo_g431rb = {
    label = "Nucleo G431RB (STM32G431RB)",
    zephyr = "nucleo_g431rb",
    openocd = { board = "board/st_nucleo_g4.cfg" },
  },
  nucleo_g474re = {
    label = "Nucleo G474RE (STM32G474RE)",
    zephyr = "nucleo_g474re",
    openocd = { board = "board/st_nucleo_g4.cfg" },
  },
  nucleo_h723zg = {
    label = "Nucleo H723ZG (STM32H723ZG)",
    zephyr = "nucleo_h723zg",
    openocd = { board = "board/st_nucleo_h7.cfg" },
  },
  nucleo_h743zi = {
    label = "Nucleo H743ZI (STM32H743ZI)",
    zephyr = "nucleo_h743zi",
    openocd = { board = "board/st_nucleo_h7.cfg" },
  },
  nucleo_h745zi_q = {
    label = "Nucleo H745ZI-Q (STM32H745ZI)",
    zephyr = "nucleo_h745zi_q",
    openocd = { board = "board/st_nucleo_h7.cfg" },
  },
  nucleo_l476rg = {
    label = "Nucleo L476RG (STM32L476RG)",
    zephyr = "nucleo_l476rg",
    openocd = { board = "board/st_nucleo_l4.cfg" },
  },
  nucleo_l496zg = {
    label = "Nucleo L496ZG (STM32L496ZG)",
    zephyr = "nucleo_l496zg",
    openocd = { board = "board/st_nucleo_l4.cfg" },
  },
  nucleo_u575zi_q = {
    label = "Nucleo U575ZI-Q (STM32U575ZI)",
    zephyr = "nucleo_u575zi_q",
    openocd = { board = "board/st_nucleo_u5.cfg" },
  },
  nucleo_wba55cg = {
    label = "Nucleo WBA55CG (STM32WBA55CG)",
    zephyr = "nucleo_wba55cg",
    openocd = { board = "board/st_nucleo_wba.cfg" },
  },
  stm32f405 = {
    label = "STM32F405 (generic ST-Link SWD)",
    openocd = {
      adapter_speed = 4000,
      interface = "interface/stlink.cfg",
      transport = "hla_swd",
      target = "target/stm32f4x.cfg",
    },
  },
}

local BOARD_ALIASES = {
  f401re = "nucleo_f401re",
  f411re = "nucleo_f411re",
  f429zi = "nucleo_f429zi",
  f446re = "nucleo_f446re",
  f303re = "nucleo_f303re",
  f334r8 = "nucleo_f334r8",
  f722ze = "nucleo_f722ze",
  f746zg = "nucleo_f746zg",
  f767zi = "nucleo_f767zi",
  g071rb = "nucleo_g071rb",
  g431rb = "nucleo_g431rb",
  g474re = "nucleo_g474re",
  h723zg = "nucleo_h723zg",
  h743zi = "nucleo_h743zi",
  h745zi = "nucleo_h745zi_q",
  l476rg = "nucleo_l476rg",
  l496zg = "nucleo_l496zg",
  u575zi = "nucleo_u575zi_q",
  wba55cg = "nucleo_wba55cg",
  f405 = "stm32f405",
}

function M.trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.board_ids()
  local out = {}
  for id in pairs(M.BOARDS) do
    out[#out + 1] = id
  end
  table.sort(out)
  return out
end

function M.normalize_board(arg)
  local b = M.trim(arg):lower():gsub("-", "_")
  if b == "" then
    return nil, "board name required"
  end
  if M.BOARDS[b] then
    return b
  end
  if BOARD_ALIASES[b] then
    return BOARD_ALIASES[b]
  end
  return nil, b
end

function M.normalize_profile(arg)
  local p = M.trim(arg):lower()
  if p == "" then
    return "baremetal"
  end
  if PROFILE_ALIASES[p] then
    return PROFILE_ALIASES[p]
  end
  return nil, p
end

function M.parse_board_and_profile(arg1, arg2)
  local a1 = M.trim(arg1)
  local a2 = M.trim(arg2)

  if a2 ~= "" then
    local board, eb = M.normalize_board(a1)
    local profile, ep = M.normalize_profile(a2)
    if not board then
      return nil, nil, "unknown board " .. tostring(eb)
    end
    if not profile then
      return nil, nil, "unknown profile " .. tostring(ep)
    end
    return board, profile
  end

  if a1 ~= "" then
    local board, eb = M.normalize_board(a1)
    if board then
      return board, M.normalize_profile("")
    end
    local profile, ep = M.normalize_profile(a1)
    if profile then
      local def = os.getenv("GDBFORGE_STM32_BOARD") or ""
      local board2, eb2 = M.normalize_board(def)
      if not board2 then
        return nil, nil, "profile given but GDBFORGE_STM32_BOARD is not set (" .. tostring(eb2) .. ")"
      end
      return board2, profile
    end
    return nil, nil, "unknown board or profile: " .. tostring(eb or ep or a1)
  end

  local def = os.getenv("GDBFORGE_STM32_BOARD") or ""
  local board, eb = M.normalize_board(def)
  if not board then
    return nil, nil, "missing board — pass board as 1st arg or export GDBFORGE_STM32_BOARD"
  end
  return board, M.normalize_profile("")
end

function M.openocd_rtos(profile)
  profile = M.normalize_profile(profile)
  if profile == "baremetal" then
    return ""
  end
  if profile == "zephyr" then
    return "Zephyr"
  end
  if profile == "freertos" then
    return "FreeRTOS"
  end
  return ""
end

function M.shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

function M.openocd_cfg_lines(def)
  local lines = {}
  local o = def.openocd or {}
  if o.board then
    lines[#lines + 1] = "source [find " .. o.board .. "]"
  else
    if o.adapter_speed then
      lines[#lines + 1] = "adapter speed " .. tostring(o.adapter_speed)
    end
    if o.interface then
      lines[#lines + 1] = "source [find " .. o.interface .. "]"
    end
    if o.transport then
      lines[#lines + 1] = "transport select " .. o.transport
    end
    if o.target then
      lines[#lines + 1] = "source [find " .. o.target .. "]"
    end
  end
  lines[#lines + 1] = ""
  lines[#lines + 1] = "$_TARGETNAME configure -event gdb-attach {"
  lines[#lines + 1] = '\techo "Debugger attaching: halting execution"'
  lines[#lines + 1] = "\treset halt"
  lines[#lines + 1] = "\tgdb_breakpoint_override hard"
  lines[#lines + 1] = "}"
  lines[#lines + 1] = ""
  lines[#lines + 1] = "$_TARGETNAME configure -event gdb-detach {"
  lines[#lines + 1] = '\techo "Debugger detaching: resuming execution"'
  lines[#lines + 1] = "\tresume"
  lines[#lines + 1] = "}"
  return lines
end

function M.write_cfg(path, lines)
  local body = table.concat(lines, "\n") .. "\n"
  local qpath = M.shell_quote(path)
  local script = "cat > " .. qpath .. " <<'GFEOF'\n" .. body .. "GFEOF"
  return gdbforge.system(script) == 0
end

function M.find_sidecar_cfg(board_id, script_dir)
  local home = os.getenv("HOME") or ""
  local candidates = {
    script_dir .. "/../" .. board_id .. "/" .. board_id .. "_openocd.cfg",
    script_dir .. "/" .. board_id .. "_openocd.cfg",
    script_dir .. "/../" .. board_id .. "_openocd.cfg",
    home .. "/.gdbforge/lua/" .. board_id .. "/" .. board_id .. "_openocd.cfg",
  }
  for _, p in ipairs(candidates) do
    if p ~= "" and gdbforge.system("test -f " .. M.shell_quote(p)) == 0 then
      return p
    end
  end
  return nil
end

function M.bundled_cfg(board_id, script_dir)
  local sidecar = M.find_sidecar_cfg(board_id, script_dir)
  if sidecar then
    return sidecar
  end
  local def = M.BOARDS[board_id]
  if not def then
    return nil
  end
  local path = "/tmp/gdbforge-openocd-" .. board_id .. ".cfg"
  if not M.write_cfg(path, M.openocd_cfg_lines(def)) then
    return nil
  end
  return path
end

function M.expand_tilde(path)
  path = M.trim(path)
  if path:sub(1, 1) ~= "~" then
    return path
  end
  local home = os.getenv("HOME") or ""
  if path == "~" then
    return home
  end
  if path:sub(1, 2) == "~/" then
    return home .. path:sub(2)
  end
  return path
end

function M.zephyr_base()
  local zb = M.expand_tilde(os.getenv("ZEPHYR_BASE") or "")
  if zb == "" then
    return nil
  end
  return zb
end

function M.zephyr_app_dir()
  return M.expand_tilde(os.getenv("PWD") or "")
end

function M.zephyr_board_support(board_id)
  local def = M.BOARDS[board_id]
  if not def or not def.zephyr or def.zephyr == "" then
    return ""
  end
  local zb = M.zephyr_base()
  if not zb then
    return ""
  end
  return zb .. "/boards/arm/" .. def.zephyr .. "/support"
end

function M.resolve_zephyr_openocd(board_id)
  local def = M.BOARDS[board_id]
  if not def or not def.zephyr or def.zephyr == "" then
    return nil, nil, "board " .. board_id .. " has no Zephyr support in catalog"
  end
  local zb = M.zephyr_base()
  if not zb then
    return nil, nil, "ZEPHYR_BASE is not set"
  end
  local support = zb .. "/boards/arm/" .. def.zephyr .. "/support"
  local cfg = support .. "/openocd.cfg"
  if gdbforge.system("test -f " .. M.shell_quote(cfg)) ~= 0 then
    return nil, nil, "Zephyr OpenOCD cfg not found: " .. cfg
  end
  return support, cfg, nil
end

function M.check_zephyr(profile, board_id)
  profile = M.normalize_profile(profile)
  if profile ~= "zephyr" then
    return true
  end
  local def = M.BOARDS[board_id]
  if not def or not def.zephyr or def.zephyr == "" then
    gdbforge.print("ERROR: board " .. board_id .. " has no Zephyr board name in catalog")
    return false
  end
  local zb = M.zephyr_base()
  if not zb then
    gdbforge.print("ERROR: zephyr profile requires ZEPHYR_BASE")
    gdbforge.print("  export ZEPHYR_BASE=~/alyn/zephyr-3.4.99/zephyr")
    return false
  end
  local scripts, cfg, err = M.resolve_zephyr_openocd(board_id)
  if not scripts then
    gdbforge.print("ERROR: " .. tostring(err))
    gdbforge.print("  export ZEPHYR_BASE=~/alyn/zephyr-3.4.99/zephyr")
    return false
  end
  local app = M.zephyr_app_dir()
  gdbforge.print("Zephyr: ZEPHYR_BASE=" .. zb)
  gdbforge.print("  OpenOCD support: " .. scripts)
  gdbforge.print("  OpenOCD cfg:     " .. cfg)
  if app ~= "" then
    gdbforge.print("  GDB app dir:     " .. app .. "  (from $PWD)")
  end
  return true
end

-- baremetal: bundled board cfg only (ignore ZEPHYR_BASE).
-- zephyr: OpenOCD paths from ZEPHYR_BASE + board catalog.
-- freertos: bundled cfg + system OpenOCD scripts.
function M.resolve_openocd_paths(profile, opts)
  opts = opts or {}
  profile = M.normalize_profile(profile)
  local bundled_cfg = opts.bundled_cfg or ""
  local board_id = opts.board_id or ""

  if profile == "zephyr" then
    local scripts, cfg, err = M.resolve_zephyr_openocd(board_id)
    if scripts then
      return scripts, cfg
    end
    return "", bundled_cfg
  end

  local scripts = ""
  local cfg = bundled_cfg

  if profile == "baremetal" then
    for _, p in ipairs({"/usr/share/openocd/scripts", "/usr/local/share/openocd/scripts"}) do
      if p ~= "" and gdbforge.system("test -d " .. M.shell_quote(p)) == 0 then
        scripts = p
        break
      end
    end
    return scripts, cfg
  end

  -- freertos
  for _, p in ipairs({"/usr/share/openocd/scripts", "/usr/local/share/openocd/scripts"}) do
    if p ~= "" and gdbforge.system("test -d " .. M.shell_quote(p)) == 0 then
      scripts = p
      break
    end
  end
  return scripts, cfg
end

function M.gdb_setup(profile)
  profile = M.normalize_profile(profile)
  if profile ~= "zephyr" then
    return
  end
  local zb = M.zephyr_base()
  if zb then
    gdbforge.gdb("dir " .. zb)
  end
  local app = M.zephyr_app_dir()
  if app ~= "" then
    gdbforge.gdb("dir " .. app)
  end
end

function M.zephyr_help_lines()
  gdbforge.print("Zephyr (profile zephyr) — required before gdbforge:")
  gdbforge.print("  export ZEPHYR_BASE=~/alyn/zephyr-3.4.99/zephyr")
  gdbforge.print("")
  gdbforge.print("OpenOCD cfg and GDB app dir are derived from ZEPHYR_BASE and $PWD.")
  gdbforge.print("Build needs CONFIG_DEBUG_THREAD_INFO=y for info threads.")
end

function M.profile_help_lines(script)
  script = script or "stm32-stlink"
  gdbforge.print("Profile (2nd arg, optional — default baremetal):")
  gdbforge.print("  baremetal   no OpenOCD -rtos (default)")
  gdbforge.print("  zephyr      requires export ZEPHYR_BASE=~/path/to/zephyr")
  gdbforge.print("  freertos    OpenOCD -rtos FreeRTOS")
  gdbforge.print("")
  gdbforge.print("Examples:")
  gdbforge.print("  :lua " .. script .. " nucleo_f429zi")
  gdbforge.print("  :lua " .. script .. " nucleo_f411re zephyr")
  gdbforge.print("  :lua " .. script .. " f429zi baremetal")
end

function M.board_help_lines(script_dir)
  gdbforge.print("Board / MCU (1st arg — names match lua/stm32/<board>/ folders and *_openocd.cfg):")
  local seen = {}
  local function show(id)
    if seen[id] then return end
    seen[id] = true
    local def = M.BOARDS[id]
    if def then
      gdbforge.print("  " .. id .. "  —  " .. (def.label or id))
    else
      gdbforge.print("  " .. id .. "  —  (sidecar openocd cfg)")
    end
  end
  for _, id in ipairs(M.board_ids()) do
    show(id)
  end
  if script_dir and script_dir ~= "" then
    for _, id in ipairs(M.file_board_ids(script_dir)) do
      if not M.BOARDS[id] then
        show(id)
      end
    end
  end
end

-- Boards discovered from lua/stm32/<id>/*_openocd.cfg beside the script tree.
function M.file_board_ids(script_dir)
  local out = {}
  local seen = {}
  if not script_dir or script_dir == "" then
    return out
  end
  local root = script_dir .. "/.."
  local code, listing = gdbforge.system(
    "find " .. M.shell_quote(root) .. " -maxdepth 2 -name '*_openocd.cfg' 2>/dev/null | sort")
  if code ~= 0 or listing == "" then
    return out
  end
  for line in listing:gmatch("[^\r\n]+") do
    local id = line:match("/([^/]+)_openocd%.cfg$")
    if id and id ~= "" and not seen[id] then
      seen[id] = true
      out[#out + 1] = id
    end
  end
  table.sort(out)
  return out
end

function M.complete_profile(token)
  token = M.trim(token):lower()
  local out = {}
  local seen = {}
  local function add(v)
    if v == "" or seen[v] then
      return
    end
    seen[v] = true
    out[#out + 1] = v
  end
  for _, h in ipairs({ "help", "-h", "--help" }) do
    if token == "" or h:sub(1, #token) == token then
      add(h)
    end
  end
  for _, p in ipairs(M.PROFILES) do
    if token == "" or p:sub(1, #token) == token then
      add(p)
    end
  end
  for alias, canon in pairs(PROFILE_ALIASES) do
    if alias ~= canon and (token == "" or alias:sub(1, #token) == token) then
      add(canon)
    end
  end
  table.sort(out)
  return out
end

function M.complete_board(token, script_dir)
  token = M.trim(token):lower()
  local out = {}
  local seen = {}
  local function add(v)
    if v == "" or seen[v] then
      return
    end
    seen[v] = true
    out[#out + 1] = v
  end
  for _, h in ipairs({ "help", "-h", "--help" }) do
    if token == "" or h:sub(1, #token) == token then
      add(h)
    end
  end
  for _, id in ipairs(M.board_ids()) do
    if token == "" or id:sub(1, #token) == token then
      add(id)
    end
  end
  if script_dir and script_dir ~= "" then
    for _, id in ipairs(M.file_board_ids(script_dir)) do
      if token == "" or id:sub(1, #token) == token then
        add(id)
      end
    end
  end
  for alias in pairs(BOARD_ALIASES) do
    if token == "" or alias:sub(1, #token) == token then
      add(alias)
    end
  end
  table.sort(out)
  return out
end

function M.complete_stlink(index, token, prior1, script_dir)
  index = tonumber(index) or 1
  prior1 = M.trim(prior1 or "")
  if index == 1 then
    if token ~= "" and M.normalize_board(token) then
      return M.complete_profile("")
    end
    return M.complete_board(token, script_dir)
  end
  if index == 2 then
    local board = M.normalize_board(prior1)
    if board then
      local out = M.complete_profile(token)
      if token ~= "" and #out == 0 then
        return M.complete_profile("")
      end
      return out
    end
  end
  return {}
end

function M.register_complete(fn)
  if gdbforge.complete_args then
    gdbforge.complete_args(fn)
  else
    complete_arg = fn
  end
end

function M.complete_arg(token)
  return M.complete_profile(token)
end

function M.init_stlink_command(script_name, script_dir)
  script_name = script_name or "stm32-stlink"
  script_dir = script_dir or gdbforge.lua_dir()
  M.register_complete(function(token, index, prior1)
    return M.complete_stlink(index, token, prior1, script_dir)
  end)
  return {
    help = function()
      gdbforge.print(script_name .. " — STM32 / Nucleo ST-Link + OpenOCD")
      gdbforge.print("Usage: :lua " .. script_name .. " <board|mcu> [baremetal|zephyr|freertos]")
      gdbforge.print("")
      M.board_help_lines(script_dir)
      gdbforge.print("")
      M.profile_help_lines(script_name)
      gdbforge.print("")
      M.zephyr_help_lines()
      gdbforge.print("")
      gdbforge.print("Default profile: baremetal (omit 2nd arg)")
      gdbforge.print("MCU aliases: f429zi → nucleo_f429zi, f405 → stm32f405, …")
    end,
    main = function(board, profile)
      local b, p, err = M.parse_board_and_profile(board, profile)
      if not b then
        gdbforge.print("ERROR: " .. tostring(err))
        gdbforge.print("Try: :lua " .. script_name .. " help")
        return
      end
      M.run_stlink({
        board = b,
        profile = p,
        script_name = script_name,
        script_dir = script_dir,
      })
    end,
  }
end

function M.load_common_from(script_dir)
  local tried = {}
  local function try(path)
    if path == "" or tried[path] then
      return nil
    end
    tried[path] = true
    if gdbforge.system("test -f " .. M.shell_quote(path)) ~= 0 then
      return nil
    end
    local fn, err = loadfile(path)
    if not fn then
      error(err)
    end
    return fn()
  end
  local home = os.getenv("HOME") or ""
  return try(script_dir .. "/../stm32_common.lua")
    or try(script_dir .. "/stm32_common.lua")
    or try(home .. "/.gdbforge/lua/stm32_common.lua")
    or error("stm32_common.lua not found — cp lua/stm32/stm32_common.lua .gdbforge/lua/")
end

function M.load_from(script_dir)
  return M.load_common_from(script_dir)
end

function M.stop_openocd()
  gdbforge.print("stopping existing openocd (if any) …")
  gdbforge.gdb("disconnect")
  gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; " ..
    "pkill -TERM -x openocd 2>/dev/null; sleep 0.2; " ..
    "pkill -KILL -x openocd 2>/dev/null; " ..
    "sleep 0.2"
  )
end

function M.spawn_openocd(opts)
  opts = opts or {}
  local openocd = opts.openocd or "openocd"
  local cfg = opts.cfg or ""
  local scripts = opts.scripts or ""
  local target = opts.target or "_TARGETNAME"
  local log = opts.log or "/tmp/gdbforge-openocd.log"
  local profile = M.normalize_profile(opts.profile)
  local rtos = M.openocd_rtos(profile)

  local cmd = { M.shell_quote(openocd) }
  if scripts ~= "" then
    cmd[#cmd + 1] = "-s"
    cmd[#cmd + 1] = M.shell_quote(scripts)
  end
  cmd[#cmd + 1] = "-f"
  cmd[#cmd + 1] = M.shell_quote(cfg)
  if rtos ~= "" then
    cmd[#cmd + 1] = "-c"
    cmd[#cmd + 1] = M.shell_quote("$" .. target .. " configure -rtos " .. rtos)
  end
  cmd[#cmd + 1] = "-c"
  cmd[#cmd + 1] = "init"
  cmd[#cmd + 1] = "-c"
  cmd[#cmd + 1] = "targets"
  cmd[#cmd + 1] = "-c"
  cmd[#cmd + 1] = M.shell_quote("reset init")

  local start = "nohup " .. table.concat(cmd, " ") ..
    " >" .. M.shell_quote(log) .. " 2>&1 </dev/null & echo $!"
  local code, out = gdbforge.system(start)
  if code ~= 0 then
    gdbforge.print("ERROR: failed to start openocd: " .. tostring(out))
    return false
  end
  local pid = (out or ""):match("(%d+)")
  if pid then
    gdbforge.print("openocd pid " .. pid .. "  log: " .. log)
  end
  return true
end

function M.default_gdb_attach(profile, port)
  port = port or "3333"
  profile = M.normalize_profile(profile)
  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set pagination off")
  gdbforge.gdb("set architecture arm")
  M.gdb_setup(profile)
  gdbforge.gdb("target remote localhost:" .. port)
  gdbforge.gdb("monitor reset halt")
  gdbforge.gdb("break main")
  gdbforge.gdb("continue")
end

function M.run_stlink(opts)
  opts = opts or {}
  local board_id = opts.board
  local profile = opts.profile or "baremetal"
  local script_name = opts.script_name or "stm32-stlink"
  local script_dir = opts.script_dir or gdbforge.lua_dir()

  local def = M.BOARDS[board_id]
  if not def then
    gdbforge.print("ERROR: unknown board " .. tostring(board_id))
    return
  end

  profile = M.normalize_profile(profile)
  if not M.check_zephyr(profile, board_id) then
    return
  end

  local bundled = M.bundled_cfg(board_id, script_dir)
  if not bundled then
    gdbforge.print("ERROR: no OpenOCD cfg for board " .. board_id)
    return
  end

  local scripts, cfg = M.resolve_openocd_paths(profile, {
    bundled_cfg = bundled,
    board_id = board_id,
  })
  if gdbforge.system("test -f " .. M.shell_quote(cfg)) ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. cfg)
    return
  end

  local openocd = os.getenv("GDBFORGE_OPENOCD") or "openocd"
  local port = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"
  local target = os.getenv("GDBFORGE_OPENOCD_TARGET") or "_TARGETNAME"
  local log = os.getenv("GDBFORGE_OPENOCD_LOG") or "/tmp/gdbforge-openocd.log"
  local rtos = M.openocd_rtos(profile)

  gdbforge.print("board: " .. board_id .. " (" .. (def.label or board_id) .. ")")
  gdbforge.print("profile: " .. profile .. "  cfg: " .. cfg .. "  port: " .. port ..
    (rtos ~= "" and ("  rtos: " .. rtos) or "  rtos: (none)"))
  M.stop_openocd()

  gdbforge.print("starting openocd …")
  if not M.spawn_openocd({
    openocd = openocd,
    cfg = cfg,
    scripts = scripts,
    target = target,
    log = log,
    profile = profile,
  }) then
    return
  end

  gdbforge.print("waiting for port " .. port .. " …")
  if not gdbforge.wait_port(port, 20) then
    gdbforge.print("ERROR: OpenOCD did not listen on :" .. port .. " — see " .. log)
    return
  end

  if opts.gdb_attach then
    opts.gdb_attach(profile, port)
  else
    M.default_gdb_attach(profile, port)
  end

  gdbforge.print(script_name .. " done — halted at main")
  gdbforge.print("OpenOCD log: " .. log .. "  (stopped on :lua re-run or gdbforge exit)")
end

return M

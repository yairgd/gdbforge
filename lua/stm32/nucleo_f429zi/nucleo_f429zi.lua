-- Nucleo F429ZI / ST-Link (OpenOCD) — thin wrapper around stm32_common.run_stlink.
-- Install: cp lua/stm32/stm32_common.lua .gdbforge/lua/
--          cp -r lua/stm32/nucleo_f429zi .gdbforge/lua/
-- Usage:   :lua nucleo_f429zi [baremetal|zephyr|freertos]

local common

local function register_complete(fn)
  if gdbforge.complete_args then
    gdbforge.complete_args(fn)
  else
    complete_arg = fn
  end
end

local function load_common()
  if common then
    return common
  end
  local dir = gdbforge.lua_dir()
  local home = os.getenv("HOME") or ""
  local paths = {
    dir .. "/../stm32_common.lua",
    dir .. "/stm32_common.lua",
    home .. "/.gdbforge/lua/stm32_common.lua",
  }
  for _, path in ipairs(paths) do
    local fn, err = loadfile(path)
    if fn then
      local ok, mod = pcall(fn)
      if ok and mod then
        common = mod
        return common
      end
      if not ok then
        error(mod)
      end
    elseif err then
      error(err)
    end
  end
  return nil
end

register_complete(function(token, index)
  local c = load_common()
  if not c then
    return {}
  end
  if tonumber(index) ~= 1 then
    return {}
  end
  return c.complete_profile(token)
end)

function help()
  local c = load_common()
  if not c then
    gdbforge.print("ERROR: stm32_common.lua not found — cp lua/stm32/stm32_common.lua .gdbforge/lua/")
    return
  end
  gdbforge.print("nucleo_f429zi — Nucleo F429ZI ST-Link + OpenOCD")
  gdbforge.print("Usage: :lua nucleo_f429zi [baremetal|zephyr|freertos]")
  gdbforge.print("")
  c.profile_help_lines("nucleo_f429zi")
  gdbforge.print("")
  c.zephyr_help_lines()
  gdbforge.print("")
  gdbforge.print("Same as: :lua stm32-stlink nucleo_f429zi [profile]")
end

function main(profile)
  local c = load_common()
  if not c then
    gdbforge.print("ERROR: stm32_common.lua not found — cp lua/stm32/stm32_common.lua .gdbforge/lua/")
    return
  end
  local p, bad = c.normalize_profile(profile)
  if not p then
    gdbforge.print("ERROR: unknown profile " .. tostring(bad) ..
      " (use baremetal, zephyr, or freertos)")
    return
  end
  c.run_stlink({
    board = "nucleo_f429zi",
    profile = p,
    script_name = "nucleo_f429zi",
    script_dir = gdbforge.lua_dir(),
  })
end

-- STM32 / Nucleo ST-Link + OpenOCD (generic entry).
-- Install: cp lua/stm32/stm32_common.lua .gdbforge/lua/
--          cp -r lua/stm32/stm32-stlink .gdbforge/lua/
-- Usage:   :lua stm32-stlink <board|mcu> [baremetal|zephyr|freertos]
--
-- Board names match lua/stm32/<board>/ folders (and *_openocd.cfg sidecars).
-- Profile defaults to baremetal when omitted.

local SCRIPT_NAME = "stm32-stlink"
local SCRIPT_DIR = gdbforge.lua_dir()

local common
local handlers

local function get_common()
  if common then
    return common
  end
  local home = os.getenv("HOME") or ""
  local paths = {
    SCRIPT_DIR .. "/../stm32_common.lua",
    SCRIPT_DIR .. "/stm32_common.lua",
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
  error("stm32_common.lua not found — cp lua/stm32/stm32_common.lua .gdbforge/lua/")
end

local function get_handlers()
  if handlers then
    return handlers
  end
  handlers = get_common().init_stlink_command(SCRIPT_NAME, SCRIPT_DIR)
  return handlers
end

get_handlers()

function help()
  get_handlers().help()
end

function main(board, profile)
  get_handlers().main(board, profile)
end

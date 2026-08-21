-- Alias of nucleo_f429zi (same logic, different :lua command name).
dofile((function()
  local dir = gdbforge.lua_dir()
  return dir .. "/nucleo_f429zi.lua"
end)())

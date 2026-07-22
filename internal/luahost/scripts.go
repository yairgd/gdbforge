package luahost

import _ "embed"

//go:embed scripts/snake.lua
var SnakeScript string

//go:embed scripts/tetris.lua
var TetrisScript string

// ScratchScript is a minimal pane for :b lua demos / gdbforge.print.
const ScratchScript = `
gdbforge.print("gdbforge Lua scratch — :lua hello")
gdbforge.register("hello", function(...)
  local n = select("#", ...)
  local parts = {"hello"}
  for i = 1, n do
    parts[#parts + 1] = tostring(select(i, ...))
  end
  gdbforge.print(table.concat(parts, " "))
end)

function on_key(k)
  gdbforge.print("key: " .. tostring(k))
end

function on_draw()
  pane.clear()
  local msg = "Lua scratch (type keys; Esc leaves ModeLua)"
  for i = 1, #msg do
    pane.set_cell(i - 1, 0, msg:sub(i, i), "cyan")
  end
end
`

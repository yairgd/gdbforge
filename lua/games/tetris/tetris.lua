-- Tetris for gdbforge LuaWidget (:b tetris).
-- Also installable: cp -r scripts/games .gdbforge/lua/  →  :lua tetris
-- Keys: h/l or arrows move, k/Up rotate, j/Down soft drop, space hard drop.

local COLS, ROWS = 10, 18
local board = {}
local piece = nil
local px, py = 0, 0
local rot = 0
local score = 0
local dead = false
local acc = 0
local fall = 0.45

local SHAPES = {
  I = { {{0,1},{1,1},{2,1},{3,1}}, {{2,0},{2,1},{2,2},{2,3}} },
  O = { {{1,0},{2,0},{1,1},{2,1}} },
  T = { {{1,0},{0,1},{1,1},{2,1}}, {{1,0},{1,1},{2,1},{1,2}},
        {{0,1},{1,1},{2,1},{1,2}}, {{1,0},{0,1},{1,1},{1,2}} },
  S = { {{1,0},{2,0},{0,1},{1,1}}, {{1,0},{1,1},{2,1},{2,2}} },
  Z = { {{0,0},{1,0},{1,1},{2,1}}, {{2,0},{1,1},{2,1},{1,2}} },
  J = { {{0,0},{0,1},{1,1},{2,1}}, {{1,0},{2,0},{1,1},{1,2}},
        {{0,1},{1,1},{2,1},{2,2}}, {{1,0},{1,1},{0,2},{1,2}} },
  L = { {{2,0},{0,1},{1,1},{2,1}}, {{1,0},{1,1},{1,2},{2,2}},
        {{0,1},{1,1},{2,1},{0,2}}, {{0,0},{1,0},{1,1},{1,2}} },
}
local NAMES = {"I","O","T","S","Z","J","L"}
local COLORS = {I="cyan", O="yellow", T="magenta", S="green", Z="red", J="blue", L="white"}

local function reset_board()
  board = {}
  for y = 0, ROWS - 1 do
    board[y] = {}
    for x = 0, COLS - 1 do
      board[y][x] = nil
    end
  end
end

local function cells(name, r)
  local frames = SHAPES[name]
  local fr = frames[(r % #frames) + 1]
  return fr
end

local function fits(name, r, ox, oy)
  local fr = cells(name, r)
  for _, c in ipairs(fr) do
    local x, y = ox + c[1], oy + c[2]
    if x < 0 or x >= COLS or y < 0 or y >= ROWS then return false end
    if board[y][x] then return false end
  end
  return true
end

local function spawn()
  local name = NAMES[math.random(1, #NAMES)]
  piece = name
  rot = 0
  px, py = 3, 0
  if not fits(piece, rot, px, py) then
    dead = true
    gdbforge.print("game over — score " .. score .. " (r restart)")
  end
end

local function lock()
  local fr = cells(piece, rot)
  for _, c in ipairs(fr) do
    local x, y = px + c[1], py + c[2]
    if y >= 0 and y < ROWS and x >= 0 and x < COLS then
      board[y][x] = COLORS[piece] or "white"
    end
  end
  -- clear lines
  local y = ROWS - 1
  while y >= 0 do
    local full = true
    for x = 0, COLS - 1 do
      if not board[y][x] then full = false break end
    end
    if full then
      score = score + 100
      for yy = y, 1, -1 do
        for x = 0, COLS - 1 do
          board[yy][x] = board[yy - 1][x]
        end
      end
      for x = 0, COLS - 1 do board[0][x] = nil end
    else
      y = y - 1
    end
  end
  spawn()
end

local function soft_drop()
  if dead or not piece then return end
  if fits(piece, rot, px, py + 1) then
    py = py + 1
  else
    lock()
  end
end

local function hard_drop()
  if dead or not piece then return end
  while fits(piece, rot, px, py + 1) do
    py = py + 1
  end
  lock()
end

math.randomseed(os.time())
reset_board()
spawn()
gdbforge.print("Tetris — h/l move, k rotate, j soft, space hard; Esc leaves")

function on_key(k)
  if dead then
    if k == "r" then
      score = 0
      dead = false
      reset_board()
      spawn()
      gdbforge.print("restarted")
    end
    return
  end
  if not piece then return end
  if k == "h" or k == "<Left>" then
    if fits(piece, rot, px - 1, py) then px = px - 1 end
  elseif k == "l" or k == "<Right>" then
    if fits(piece, rot, px + 1, py) then px = px + 1 end
  elseif k == "k" or k == "<Up>" then
    local nr = rot + 1
    if fits(piece, nr, px, py) then rot = nr end
  elseif k == "j" or k == "<Down>" then
    soft_drop()
  elseif k == " " then
    hard_drop()
  end
end

function on_tick(dt)
  if dead then return end
  acc = acc + dt
  while acc >= fall do
    acc = acc - fall
    soft_drop()
  end
end

function on_draw()
  pane.clear()
  local pw, ph = pane.size()
  local ox = math.floor((pw - COLS) / 2)
  local oy = math.floor((ph - ROWS - 1) / 2)
  if ox < 0 then ox = 0 end
  if oy < 0 then oy = 0 end

  for y = 0, ROWS - 1 do
    for x = 0, COLS - 1 do
      local col = board[y][x]
      if col then
        pane.set_cell(ox + x, oy + y, "#", col)
      else
        pane.set_cell(ox + x, oy + y, ".", "gray")
      end
    end
  end
  if piece and not dead then
    local fr = cells(piece, rot)
    local col = COLORS[piece] or "white"
    for _, c in ipairs(fr) do
      local x, y = px + c[1], py + c[2]
      if y >= 0 then
        pane.set_cell(ox + x, oy + y, "#", col)
      end
    end
  end
  local title = "Tetris  score=" .. score
  if dead then title = title .. "  DEAD" end
  for i = 1, #title do
    if ox + i - 1 < pw and oy > 0 then
      pane.set_cell(ox + i - 1, oy - 1, title:sub(i, i), "cyan")
    end
  end
end

gdbforge.register("tetris_score", function()
  gdbforge.print("tetris score=" .. tostring(score))
end)

-- :lua tetris (when copied under .gdbforge/lua/) opens the builtin game pane.
function main()
  gdbforge.open_buffer("tetris")
end

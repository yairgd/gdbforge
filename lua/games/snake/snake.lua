-- Snake for gdbforge LuaWidget (:b snake).
-- Also installable: cp -r scripts/games .gdbforge/lua/  →  :lua snake
-- Keys: Left/Right/Up/Down arrows (Esc leaves ModeLua).

local W, H = 20, 12
local snake = { {x=5,y=5}, {x=4,y=5}, {x=3,y=5} }
local dir = {x=1, y=0}
local pending = nil
local food = {x=10, y=5}
local score = 0
local dead = false
local acc = 0
local step = 0.22

local function rand_food()
  local occupied = {}
  for _, s in ipairs(snake) do
    occupied[s.x .. "," .. s.y] = true
  end
  for _ = 1, 200 do
    local fx = math.random(0, W - 1)
    local fy = math.random(0, H - 1)
    if not occupied[fx .. "," .. fy] then
      food.x, food.y = fx, fy
      return
    end
  end
end

math.randomseed(os.time())
rand_food()
gdbforge.print("Snake — arrows; Esc leaves")

local function set_dir(dx, dy)
  if dead then return end
  -- no instant reverse
  if #snake > 1 and snake[1].x + dx == snake[2].x and snake[1].y + dy == snake[2].y then
    return
  end
  pending = {x=dx, y=dy}
end

function on_key(k)
  if k == "<Left>" then set_dir(-1, 0)
  elseif k == "<Right>" then set_dir(1, 0)
  elseif k == "<Up>" then set_dir(0, -1)
  elseif k == "<Down>" then set_dir(0, 1)
  elseif k == "r" and dead then
    snake = { {x=5,y=5}, {x=4,y=5}, {x=3,y=5} }
    dir = {x=1, y=0}
    pending = nil
    score = 0
    dead = false
    rand_food()
    gdbforge.print("restarted")
  end
end

local function step_snake()
  if dead then return end
  if pending then
    dir = pending
    pending = nil
  end
  local nx = snake[1].x + dir.x
  local ny = snake[1].y + dir.y
  if nx < 0 or ny < 0 or nx >= W or ny >= H then
    dead = true
    gdbforge.print("game over — score " .. score .. " (r restart)")
    return
  end
  for i = 1, #snake do
    if snake[i].x == nx and snake[i].y == ny then
      dead = true
      gdbforge.print("game over — score " .. score .. " (r restart)")
      return
    end
  end
  table.insert(snake, 1, {x=nx, y=ny})
  if nx == food.x and ny == food.y then
    score = score + 1
    rand_food()
  else
    table.remove(snake)
  end
end

function on_tick(dt)
  acc = acc + dt
  while acc >= step do
    acc = acc - step
    step_snake()
  end
end

function on_draw()
  pane.clear()
  local pw, ph = pane.size()
  local ox = math.floor((pw - W) / 2)
  local oy = math.floor((ph - H - 1) / 2)
  if ox < 0 then ox = 0 end
  if oy < 0 then oy = 0 end

  for y = 0, H - 1 do
    for x = 0, W - 1 do
      pane.set_cell(ox + x, oy + y, ".", "gray")
    end
  end
  pane.set_cell(ox + food.x, oy + food.y, "*", "red")
  for i, s in ipairs(snake) do
    local ch = "o"
    local col = "green"
    if i == 1 then ch, col = "@", "yellow" end
    pane.set_cell(ox + s.x, oy + s.y, ch, col)
  end
  local title = "Snake  score=" .. score
  if dead then title = title .. "  DEAD" end
  for i = 1, #title do
    local ch = title:sub(i, i)
    if ox + i - 1 < pw and oy - 1 >= 0 then
      pane.set_cell(ox + i - 1, oy - 1, ch, "cyan")
    end
  end
end

gdbforge.register("snake_score", function()
  gdbforge.print("snake score=" .. tostring(score))
end)

-- :lua snake (when copied under .gdbforge/lua/) opens the builtin game pane.
function main()
  gdbforge.open_buffer("snake")
end

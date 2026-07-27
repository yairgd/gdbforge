package luahost

import _ "embed"

//go:embed scripts/snake.lua
var SnakeScript string

//go:embed scripts/tetris.lua
var TetrisScript string

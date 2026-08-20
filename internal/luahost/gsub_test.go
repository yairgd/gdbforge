package luahost

import (
	"testing"
)

func TestGsubChain(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`
local tty = "ttyPS1"
tty = (tostring(tty or ""):gsub("^%s+", ""):gsub("%s+$", ""))
tty = tty:gsub("^/dev/", "")
if tty ~= "ttyPS1" then error("bad: "..tty) end
`, "gsub"); err != nil {
		t.Fatal(err)
	}
}

func TestGsubInFunctionSplit(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`
local function normalize_board_tty(tty)
  tty = tostring(tty or "")
  tty = tty:gsub("^%s+", "")
  tty = tty:gsub("%s+$", "")
  tty = tty:gsub("^/dev/", "")
  return tty
end
local r = normalize_board_tty("ttyPS1")
if r ~= "ttyPS1" then error(r) end
`, "fn"); err != nil {
		t.Fatal(err)
	}
}

func TestGsubInFunctionChained(t *testing.T) {
	rt := New(nil, nil)
	defer rt.Close()
	if err := rt.LoadString(`
local function normalize_board_tty(tty)
  tty = tostring(tty or "")
  tty = tty:gsub("^%s+", "")
  tty = tty:gsub("%s+$", "")
  tty = tty:gsub("^/dev/", "")
  return tty
end
normalize_board_tty("ttyPS1")
`, "fn2"); err != nil {
		t.Fatal(err)
	}
}

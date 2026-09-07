-- Step 1 only — prefer :lua kgdb_kdmx (all steps in one script).
-- Install: cp -r lua/kgdb_setup lua/kgdb_common .gdbforge/lua/

local function load_common()
  local path = gdbforge.lua_dir() .. "/../kgdb_common/kgdb_common.lua"
  local fh = io.open(path, "r")
  if not fh then
    path = "./.gdbforge/lua/kgdb_common/kgdb_common.lua"
    fh = io.open(path, "r")
  end
  if not fh then
    gdbforge.print("ERROR: kgdb_common not found — use :lua kgdb_kdmx")
    return false
  end
  fh:close()
  dofile(path)
  return true
end

function help()
  gdbforge.print("kgdb_setup — step 1 only (kgdboc via SSH)")
  gdbforge.print("Prefer: :lua kgdb_kdmx   (runs setup + kdmx + attach)")
end

function main(board_tty)
  if not load_common() then return end
  local C = kgdb_common
  board_tty = C.trim(board_tty)
  if board_tty == "" then
    board_tty = C.env("GDBFORGE_KGDB_BOARD_TTY", "ttyPS0")
  end
  local host = C.kgdb_host()
  local user = C.kgdb_user()
  local baud = C.env("GDBFORGE_KGDB_BAUD", "115200")
  local ok, detail = C.setup_kgdboc(user, host, board_tty, baud)
  if not ok then
    gdbforge.print("ERROR: " .. tostring(detail))
  end
end

-- Cleanup stale kgdb: GDB disconnect + kill kdmx + clear kgdboc (SSH).
-- Install: cp -r lua/kgdb_detach lua/kgdb_common .gdbforge/lua/
--
-- Usage:   :lua kgdb_detach
-- (also runs automatically at start of :lua kgdb_kdmx unless GDBFORGE_KGDB_CLEANUP=0)

local function load_common()
  local paths = {
    gdbforge.lua_dir() .. "/../kgdb_common/kgdb_common.lua",
    "./.gdbforge/lua/kgdb_common/kgdb_common.lua",
  }
  local home = os.getenv("HOME")
  if home and home ~= "" then
    paths[#paths + 1] = home .. "/.gdbforge/lua/kgdb_common/kgdb_common.lua"
  end
  for _, path in ipairs(paths) do
    local fh = io.open(path, "r")
    if fh then
      fh:close()
      dofile(path)
      return true
    end
  end
  gdbforge.print("ERROR: kgdb_common not found")
  return false
end

function help()
  gdbforge.print("kgdb_detach — cleanup stale kgdb (disconnect + kdmx + clear kgdboc)")
  gdbforge.print("Usage: :lua kgdb_detach")
  gdbforge.print("Runs automatically before :lua kgdb_kdmx (GDBFORGE_KGDB_CLEANUP=1)")
end

function main()
  if not load_common() then return end
  kgdb_common.kgdb_cleanup()
end

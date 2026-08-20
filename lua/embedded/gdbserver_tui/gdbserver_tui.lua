-- gdbserver in an external terminal — best for local host TUI apps (GDB).
-- Install: cp -r lua/embedded/gdbserver_tui .gdbforge/lua/
-- Env:     GDBFORGE_TERMINAL=mate-terminal|kitty|xterm|…
-- Usage:   :lua gdbserver_tui [prog] [port]
--
-- Example:
--   :lua gdbserver_tui ./bin/gdbforge 2345
-- Inferior stdio is the external terminal. For a remote board use :lua remotegdb.

function help()
  gdbforge.print("gdbserver_tui — local gdbserver in external terminal + target remote")
  gdbforge.print("Usage: :lua gdbserver_tui [prog] [port]")
  gdbforge.print("  :lua gdbserver_tui ./bin/gdbforge 2345")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("Defaults: prog=./hello port=2345")
  gdbforge.print("Remote board: use :lua remotegdb")
end

function main(prog, port)
  prog = prog or "./hello"
  port = port or "2345"

  gdbforge.print("spawning gdbserver in external terminal …")
  gdbforge.spawn_terminal("gdbserver", ":" .. port, prog)

  gdbforge.print("waiting for :" .. port)
  if not gdbforge.wait_port(port, 15) then
    gdbforge.print("ERROR: gdbserver did not listen — check the other window")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("target remote localhost:" .. port)
  gdbforge.print("attached — inferior TUI lives in the gdbserver terminal")
end

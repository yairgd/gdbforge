-- gdbserver in an external terminal (pattern A) — best for TUI apps.
-- Install: cp -r scripts/gdbserver_tui .gdbforge/lua/
-- Usage:   :lua gdbserver_tui [prog] [port]
--
-- Example:
--   :lua gdbserver_tui ./bin/gdbforge 2345
-- Then continue / breakpoints as usual. Inferior stdio is the
-- external terminal (full VT), not :b io.

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

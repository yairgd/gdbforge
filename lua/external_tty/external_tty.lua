-- External tty for TUI / full-VT inferiors (GDB).
-- Install: cp -r lua/external_tty .gdbforge/lua/
-- Env:     GDBFORGE_TERMINAL=mate-terminal|kitty|xterm|…
-- Usage:   :lua external_tty
--
-- Opens a real terminal, points GDB -inferior-tty-set at that pts.
-- Delve: prefer :lua dlv_ext_port (see lua/README.md).

function help()
  gdbforge.print("external_tty — open a real terminal and set GDB inferior-tty")
  gdbforge.print("Usage: :lua external_tty")
  gdbforge.print("Env: GDBFORGE_TERMINAL")
  gdbforge.print("Delve: prefer :lua dlv_ext_port")
end

function main()
  local pts = gdbforge.open_external_tty()
  gdbforge.print("using " .. pts)
  gdbforge.set_inferior_tty(pts)
  gdbforge.print("inferior stdio → external terminal; :b io shows a note only")
  gdbforge.print("break / run as usual — the program UI appears in the other window")
end

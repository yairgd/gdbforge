-- External tty for TUI / full-VT inferiors (pattern B).
-- Install: cp -r scripts/external_tty .gdbforge/lua/
-- Usage:   :lua external_tty
--
-- Opens a real terminal (kitty/xterm/…; override with GDBFORGE_TERMINAL),
-- points GDB -inferior-tty-set at that pts, and leaves :b io idle.
-- Delve cannot change --tty after start — use GDBFORGE_INFERIOR_TTY=… at
-- launch, or :lua dlv_tui / :lua terminal_debug (see scripts/README.md).

function main()
  local pts = gdbforge.open_external_tty()
  gdbforge.print("using " .. pts)
  gdbforge.set_inferior_tty(pts)
  gdbforge.print("inferior stdio → external terminal; :b io shows a note only")
  gdbforge.print("break / run as usual — the program UI appears in the other window")
end

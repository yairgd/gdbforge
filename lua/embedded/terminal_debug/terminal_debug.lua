-- Debug a program with stdio in another real terminal (GDB).
--
-- Install: cp -r lua/embedded/terminal_debug .gdbforge/lua/
--
-- Env:
--   GDBFORGE_TERMINAL=mate-terminal|kitty|xterm|gnome-terminal|…
--   GDBFORGE_INFERIOR_TTY=/dev/pts/N   (optional startup override)
--
-- Usage:
--   gdbforge ./myprog
--   :lua terminal_debug              -- open external tty + route inferior stdio
--   :lua terminal_debug ./myprog     -- also: file ./myprog, break main
--   :lua terminal_debug ./myprog run -- also run (after break main)
--
-- For Go + Delve prefer :lua dlv_ext_port (see lua/README.md).

function help()
  gdbforge.print("terminal_debug — external tty for inferior stdio (GDB)")
  gdbforge.print("Usage: :lua terminal_debug [prog] [run]")
  gdbforge.print("  :lua terminal_debug ./myprog")
  gdbforge.print("  :lua terminal_debug ./myprog run")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("  export GDBFORGE_INFERIOR_TTY=/dev/pts/N")
  gdbforge.print("For Go + Delve prefer :lua dlv_ext_port")
end

function main(prog, action)
  local pts = gdbforge.open_external_tty()
  gdbforge.print("external tty: " .. pts)
  gdbforge.set_inferior_tty(pts)
  gdbforge.print("inferior stdio → that terminal; :b io shows a note only")

  if prog == nil or prog == "" then
    gdbforge.print("ready — set file / break / run yourself, or:")
    gdbforge.print("  :lua terminal_debug ./myprog [run]")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("file " .. prog)
  gdbforge.gdb("break main")
  gdbforge.print("file " .. prog .. " + break main")

  if action == "run" or action == "r" then
    gdbforge.gdb("run")
    gdbforge.print("run — watch the other terminal for program I/O")
  else
    gdbforge.print("next: run  (or :lua terminal_debug " .. prog .. " run)")
  end
end

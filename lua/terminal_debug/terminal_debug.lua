-- Debug a program with stdio in another real terminal (GDB or Go/Delve).
--
-- Install:
--   mkdir -p .gdbforge/lua
--   cp -r scripts/terminal_debug .gdbforge/lua/
--
-- Usage inside gdbforge:
--   :lua terminal_debug              -- open external tty + route inferior stdio
--   :lua terminal_debug ./myprog     -- also: file ./myprog, break main
--   :lua terminal_debug ./myprog run -- also run (after break main)
--
-- GDB (C/C++/Rust, or Go built with gccgo / dwarf):
--   gdbforge -- ./myprog
--   :lua terminal_debug ./myprog
--   then n / s / c as usual — the program UI is in the other window.
--
-- Go + Delve: --tty is fixed at process start. Either:
--   1) Open the hold terminal first, then:
--        GDBFORGE_INFERIOR_TTY=/dev/pts/N gdbforge -g dlv -- ./hello
--   2) Or use :lua dlv_tui ./hello  (pattern A — headless dlv in that terminal)
--
-- Terminal emulator: GDBFORGE_TERMINAL=kitty|xterm|gnome-terminal|…

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

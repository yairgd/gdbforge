-- Same as dlv_ext_port (alias).
--
-- Install: cp -r lua/dlv_port .gdbforge/lua/
-- Env:     GDBFORGE_TERMINAL=mate-terminal|kitty|xterm|…
-- Usage:
--   gdbforge -g dlv ./prog
--   :lua dlv_port
--   :lua dlv_port 12345
--   :lua dlv_port 12345 p1 p2 p3

function help()
  gdbforge.print("dlv_port — alias of dlv_ext_port (headless Delve + connect)")
  gdbforge.print("Usage: gdbforge -g dlv ./prog")
  gdbforge.print("       :lua dlv_port [port] [extra prog args…]")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_TERMINAL=mate-terminal")
  gdbforge.print("Default port: 2345")
end

function main(port, ...)
  port = tostring(port or "2345")
  local addr = "127.0.0.1:" .. port
  local prog = gdbforge.program()
  if prog == nil or prog == "" then
    gdbforge.print("ERROR: no session program — start with: gdbforge -g dlv -- ./prog")
    return
  end

  local extras = { ... }
  local msg = "program from session: " .. prog
  if #extras > 0 then
    msg = msg .. " -- " .. table.concat(extras, " ")
  end
  gdbforge.print(msg)
  gdbforge.print("opening external terminal (headless dlv on " .. addr .. ") …")
  gdbforge.spawn_dlv_headless(port, unpack(extras))

  gdbforge.print("waiting for " .. addr)
  if not gdbforge.wait_port(port, 20) then
    gdbforge.print("ERROR: nothing listening on :" .. port .. " — check the other terminal")
    return
  end

  gdbforge.print("connecting current session → " .. addr)
  gdbforge.dlv_connect(addr)
  gdbforge.open_buffer("gdb")
  gdbforge.print("connected — BPs re-applied; inferior I/O is the other window")
  gdbforge.print("next: c  (to hit main / your breakpoints). After exit: r then c")
end

-- Headless Delve on an external port, then connect the current gdbforge session.
--
-- Usage:
--   gdbforge -g dlv -- ./prog
--   :lua dlv_ext_port                     -- port 2345, no extra args
--   :lua dlv_ext_port 12345               -- custom port
--   :lua dlv_ext_port 12345 p1 p2 p3      -- port + args for ./prog
--
-- Program path comes from the session; p1..pn are appended after it:
--   dlv exec --headless … -- ./prog p1 p2 p3

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

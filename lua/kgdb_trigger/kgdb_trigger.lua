-- :lua kgdb_trigger — break in (after :lua kgdb_serial)
--
--   1. echo g > /proc/sysrq-trigger
--   2. switch UART to gdb
--
-- Manual attach (once):  (gdb) target remote <gdb PTY>
-- Back to minicom:       (gdb) continue  or  :serial-switch console

function help()
  gdbforge.print(":lua kgdb_trigger")
  gdbforge.print("  1. echo g > /proc/sysrq-trigger")
  gdbforge.print("  2. :serial-switch gdb")
  gdbforge.print("")
  gdbforge.print("First time also: (gdb) target remote <gdb PTY>")
end

function main(subcmd)
  subcmd = (subcmd or ""):match("^%s*(.-)%s*$")

  if subcmd == "console" then
    gdbforge.serial_switch_console()
    return
  end

  if gdbforge.serial_debugger_pty() == nil or gdbforge.serial_debugger_pty() == "" then
    gdbforge.print("ERROR: run :lua kgdb_serial first")
    return
  end

  pcall(gdbforge.serial_send, "echo g > /proc/sysrq-trigger")
  gdbforge.serial_switch_gdb()
  if gdbforge.set_kgdb_mode then
    gdbforge.set_kgdb_mode(true)
  end
end

-- kgdb_uart — kernel gdb over UART + kdmx. Edit this file for your board.
--
--   export GDBFORGE_KGDB_UART=/dev/ttyUSB0
--   export GDBFORGE_KGDB_VMLINUX=/path/to/vmlinux   # optional
--   :lua kgdb_uart
--
-- Flow: kgdboc + brk alias (raw UART) → kdmx → minicom → sysrq-g → target remote

local function env(k, d)
  local v = os.getenv(k)
  if v and v:match("%S") then return v:match("^%s*(.-)%s*$") end
  return d
end

local function sh(s) return "'" .. tostring(s):gsub("'", "'\\''") .. "'" end

local function uart(dev, baud, line)
  if not gdbforge.uart_send then
    error("rebuild gdbforge (needs gdbforge.uart_send)")
  end
  gdbforge.uart_send(dev, tonumber(baud) or 115200, line)
end

function help()
  gdbforge.print(":lua kgdb_uart  — set GDBFORGE_KGDB_UART first")
end

function main()
  local uart_dev = env("GDBFORGE_KGDB_UART", "")
  if uart_dev == "" then
    gdbforge.print("ERROR: export GDBFORGE_KGDB_UART=/dev/ttyUSB0")
    return
  end

  local baud = env("GDBFORGE_KGDB_BAUD", "115200")
  local board = env("GDBFORGE_KGDB_BOARD_TTY", "ttyPS0"):gsub("^/dev/", "")
  local vmlinux = env("GDBFORGE_KGDB_VMLINUX", "")
  local kdmx = env("GDBFORGE_KGDB_KDMX", "kdmx")
  local status = "/tmp/kdmx_ports"
  local brk_wait = tonumber(env("GDBFORGE_KGDB_SYSRQ_WAIT", "0.5")) or 0.5

  if gdbforge.set_kgdb_mode then gdbforge.set_kgdb_mode(true) end

  -- 1) claim UART, configure board (needs shell prompt on serial)
  gdbforge.print("setup " .. uart_dev .. " …")
  gdbforge.system("fuser -k " .. sh(uart_dev) .. " 2>/dev/null; sleep 0.5")
  uart(uart_dev, baud, "echo " .. board .. "," .. baud .. " > /sys/module/kgdboc/parameters/kgdboc")
  uart(uart_dev, baud, "alias brk='echo g > /proc/sysrq-trigger'")
  --uart(uart_dev, baud, [[alias brk='echo "Exit kgdb: (gdb) continue"; echo g > /proc/sysrq-trigger']])
  gdbforge.sleep(0.5)

  -- 2) kdmx
  gdbforge.system("rm -f " .. sh(status .. "_gdb") .. " " .. sh(status .. "_trm"))
  gdbforge.system(string.format(
    "nohup %s -n -p %s -b %s -s %s >/tmp/kdmx.log 2>&1 </dev/null & echo $!",
    sh(kdmx), sh(uart_dev), sh(baud), sh(status)))
  local gdb_pty, trm_pty
  for _ = 1, 30 do
    gdbforge.sleep(0.5)
    local _, g = gdbforge.system("cat " .. sh(status .. "_gdb") .. " 2>/dev/null")
    local _, t = gdbforge.system("cat " .. sh(status .. "_trm") .. " 2>/dev/null")
    gdb_pty = (g or ""):match("(/dev/pts/%d+)")
    trm_pty = (t or ""):match("(/dev/pts/%d+)")
    if gdb_pty and trm_pty then break end
  end
  if not gdb_pty then
    gdbforge.print("ERROR: kdmx failed — see /tmp/kdmx.log")
    return
  end
  gdbforge.print("console " .. trm_pty .. "  gdb " .. gdb_pty)

  gdbforge.spawn_terminal("minicom", "-D", trm_pty, "-o")
  gdbforge.open_buffer("gdb")
  if vmlinux ~= "" then
    gdbforge.gdb("file " .. vmlinux)
    gdbforge.sleep(tonumber(env("GDBFORGE_KGDB_SYMBOL_WAIT", "5")) or 5)
  end

  -- 3) break in, then attach
  gdbforge.gdb("set remotetimeout 60")
  gdbforge.sleep(0.5)  
  uart(trm_pty, baud, "brk")
  gdbforge.sleep(brk_wait)
  --gdbforge.print("target remote " .. gdb_pty)
  --if gdbforge.gdb_query then
  --  local _, err = gdbforge.gdb_query("target remote " .. gdb_pty, 120)
  --  if err and err ~= "" then
  --    gdbforge.print("ERROR: " .. err)
  --    return
  --  end
  --else
  -- gdbforge.gdb("target remote " .. gdb_pty)
  --end
  gdbforge.print("target remote " .. gdb_pty)
  gdbforge.gdb("target remote " .. gdb_pty)

  gdbforge.print("kgdb_uart: stopped — set breakpoints, continue")
end

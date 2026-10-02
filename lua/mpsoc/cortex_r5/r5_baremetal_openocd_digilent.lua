-- Cortex-R5 / Digilent HS2 OpenOCD bring-up.
-- Install: copy lua/mpsoc/cortex_r5 into .gdbforge/lua/ (keeps cfg + r5_target.xml)
-- Usage:   :lua r5_baremetal_openocd_digilent [baremetal|zephyr|freertos]
--
-- The profile picks the RTOS OpenOCD decodes into GDB threads. zephyr works on this core,
-- freertos does not (OpenOCD has no cortex_r4 FreeRTOS support) — see r5_common.lua.
--
-- Env:
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--   GDBFORGE_OPENOCD       path to openocd (default: openocd on PATH)
--   GDBFORGE_OPENOCD_CFG   OpenOCD config (default: script dir r5_openocd_digilent.cfg)
--   GDBFORGE_OPENOCD_PORT  GDB listen port (default 3333)
--   GDBFORGE_OPENOCD_TARGET  target name the cfg publishes, for -rtos (default _TARGETNAME)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--   GDBFORGE_R5_TCM_INIT   TCM banks to zero before load: atcm (default) | btcm | all | 0
--   ZEPHYR_BASE            kernel tree, for GDB source paths under profile zephyr
--
-- Kills any existing openocd, then gdbforge.spawn (background — Code pane stays).
-- wait_port waits until OpenOCD listens before target remote.
-- Optional: :b exec to watch OpenOCD logs.
--
-- Nothing here initialises the PS. If the board is not booting an FSBL that does it, see
-- PARK below: a companion xsdb script the user runs by hand, outside gdbforge, before this.

local C = dofile(gdbforge.lua_dir() .. "/r5_common.lua")

local OPENOCD = os.getenv("GDBFORGE_OPENOCD") or "openocd"
local CFG = os.getenv("GDBFORGE_OPENOCD_CFG")
  or (gdbforge.lua_dir() .. "/r5_openocd_digilent.cfg")
local PORT = os.getenv("GDBFORGE_OPENOCD_PORT") or "3333"
local TDESC = os.getenv("GDBFORGE_TDESC")
  or (gdbforge.lua_dir() .. "/r5_target.xml")

-- Mentioned in help(), never run from here: it drives xsdb, which needs hw_server to own the
-- JTAG cable, and openocd is holding that cable for as long as a session is open.
local PARK = "scripts/zynqmp-park-el3.sh"

-- TCM as the R5 itself addresses it. 64KB per bank per core in split mode. ATCM alone by
-- default: an image that fits in it never touches BTCM, and BTCM is not always powered or
-- even mapped, in which case writing it fails and takes the probe's memory path down with
-- it — after which everything else, including inserting a breakpoint, reports
-- "Cannot access memory".
local ATCM = { name = "ATCM", base = 0x00000000 }
local BTCM = { name = "BTCM", base = 0x00020000 }
local TCM_BANK_SETS = { atcm = { ATCM }, btcm = { BTCM }, all = { ATCM, BTCM } }
local TCM_BANK_WORDS = 16384

-- Parse GDBFORGE_R5_CORE → 0 or 1 (default 0). Accepts 0|1|R0|R1.
local function r5_core()
  local v = os.getenv("GDBFORGE_R5_CORE") or "0"
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):upper():gsub("^R", "")
  if v == "0" or v == "1" then
    return tonumber(v)
  end
  return nil, v
end

local function shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

-- GDBFORGE_R5_TCM_INIT: which banks to zero — atcm (default) | btcm | all, or
-- 0/no/off/false for none. Returns (banks, bad_value): both nil means "off on purpose".
local function tcm_banks()
  local v = os.getenv("GDBFORGE_R5_TCM_INIT")
  if v == nil then
    return TCM_BANK_SETS.atcm
  end
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):lower()
  if v == "0" or v == "no" or v == "off" or v == "false" then
    return nil, nil
  end
  if v == "" or v == "1" or v == "yes" or v == "on" or v == "true" then
    return TCM_BANK_SETS.atcm
  end
  if TCM_BANK_SETS[v] then
    return TCM_BANK_SETS[v]
  end
  return nil, v
end

-- Give every TCM word a valid ECC syndrome by writing it once.
--
-- On this core a sub-word store is a read-modify-write, and the read half of one aimed at
-- a granule that has never been written finds no valid syndrome and raises a synchronous
-- parity error. Zephyr's bss and noinit are NOBITS, so load never writes them, and
-- arch_bss_zero() data-aborts on the first byte of __bss_start. An FSBL (XFsbl_TcmEccInit)
-- or Linux remoteproc would have cleared TCM first; over JTAG nothing does.
--
-- Has to run after halt and before load: load rewrites everything this touched, which also
-- resyncs whatever gdb had cached for breakpoints sitting in that range. Run it after load
-- instead and it erases the image.
--
-- Verified rather than fired and forgotten, because a bank that is not there fails quietly
-- and the next symptom is an unrelated-looking "Cannot access memory" somewhere else.
local function tcm_ecc_init()
  local banks, bad = tcm_banks()
  if bad then
    gdbforge.print("ERROR: GDBFORGE_R5_TCM_INIT must be atcm|btcm|all|0 (got " ..
      tostring(bad) .. ") — skipping TCM init")
    return
  end
  if not banks then
    gdbforge.print("TCM ECC init skipped (GDBFORGE_R5_TCM_INIT=0)")
    return
  end
  for _, bank in ipairs(banks) do
    local cmd = string.format("set {int[%d]}0x%X = {0}", TCM_BANK_WORDS, bank.base)
    if gdbforge.gdb_query == nil then
      gdbforge.gdb(cmd)
      gdbforge.print(string.format("%s 0x%08X zeroed for ECC (unverified: gdbforge too old)",
        bank.name, bank.base))
    else
      local out, err = gdbforge.gdb_query(cmd, 60)
      out = tostring(out or "")
      if err ~= nil or out:match("Cannot access memory") or out:match("Cannot write") then
        gdbforge.print(string.format("WARNING: %s 0x%08X is not writable — %s",
          bank.name, bank.base, (tostring(err or out):gsub("%s+$", ""))))
        gdbforge.print("  Is that bank powered, and does GDBFORGE_R5_CORE match the image?")
      else
        gdbforge.print(string.format("%s 0x%08X zeroed for ECC", bank.name, bank.base))
      end
    end
  end
end

-- Stop a leftover OpenOCD so the GDB port is free before respawn.
local function stop_openocd()
  gdbforge.print("stopping existing openocd (if any) …")
  gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

-- True if a non-zombie openocd is still running.
local function openocd_alive()
  local st = gdbforge.system(
    "pids=$(pidof openocd 2>/dev/null); " ..
    "[ -z \"$pids\" ] && exit 1; " ..
    "for p in $pids; do " ..
    "  s=$(ps -o stat= -p \"$p\" 2>/dev/null | tr -d ' '); " ..
    "  case \"$s\" in Z*) ;; *) exit 0 ;; esac; " ..
    "done; exit 1"
  )
  return st == 0
end

C.register_complete(C.complete_profile_only)

function help()
  gdbforge.print("r5_baremetal_openocd_digilent — Digilent HS2 OpenOCD → load + break main")
  gdbforge.print("Usage: :lua r5_baremetal_openocd_digilent [baremetal|zephyr|freertos]")
  gdbforge.print("")
  C.profile_help_lines("r5_baremetal_openocd_digilent", "openocd")
  gdbforge.print("")
  C.zephyr_help_lines()
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  Digilent JTAG-HS2 (FTDI 0403:6014) connected to ZynqMP.")
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
  gdbforge.print("  This script only uploads your app ELF over OpenOCD (load + break main).")
  gdbforge.print("  It does NOT load FSBL or init DDR/clocks like Xilinx XSCT does.")
  gdbforge.print("")
  gdbforge.print("No FSBL running on the board? Then nothing set the clocks, PLLs and MIO,")
  gdbforge.print("and the app loads but prints nothing. Fix it before debugging, from a")
  gdbforge.print("shell, with no gdbforge session open:")
  gdbforge.print("    " .. PARK .. " -p <platform>/hw/psu_init.tcl")
  gdbforge.print("  That resets the board into JTAG boot mode — no FSBL, ATF or U-Boot runs")
  gdbforge.print("  at all — and then runs psu_init over the DAP, which is the part the R5")
  gdbforge.print("  needs; the A53 it parks at EL3 matters only for A53 bare metal. With no")
  gdbforge.print("  FSBL nothing releases the RPU from reset either, so the probe has to do")
  gdbforge.print("  that before load lands. It needs the JTAG cable to itself, which is why")
  gdbforge.print("  gdbforge cannot run it for you. Afterwards do not let the debugger reset")
  gdbforge.print("  the target: a reset discards psu_init. --clear-boot-mode to boot normally.")
  gdbforge.print("")
  gdbforge.print("ATCM is zeroed between halt and load, because the R5 aborts on the first")
  gdbforge.print("store into a TCM granule nothing has ever written: a sub-word store is a")
  gdbforge.print("read-modify-write, and the read finds no valid ECC syndrome. bss and noinit")
  gdbforge.print("are NOBITS, so load never writes them, and a Zephyr app dies in")
  gdbforge.print("arch_bss_zero() with a synchronous parity error (K_ERR_ARM_SYNC_PARITY_ERROR,")
  gdbforge.print("reason 52) before main. An FSBL or Linux remoteproc would have cleared TCM;")
  gdbforge.print("over JTAG nothing does. GDBFORGE_R5_TCM_INIT picks the banks — atcm is the")
  gdbforge.print("default, all adds BTCM at 0x20000 (only if your image uses it: writing a")
  gdbforge.print("bank that is not mapped fails and breaks later memory access), 0 disables.")
  gdbforge.print("")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_OPENOCD=" .. OPENOCD)
  gdbforge.print("  export GDBFORGE_OPENOCD_CFG=" .. CFG)
  gdbforge.print("  export GDBFORGE_OPENOCD_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("After: :b exec for OpenOCD logs")
end

function main(profile_arg)
  local core, bad = r5_core()
  if not core then
    gdbforge.print("ERROR: GDBFORGE_R5_CORE must be 0|1|R0|R1 (got " .. tostring(bad) .. ")")
    return
  end

  local profile, bad_profile = C.normalize_profile(profile_arg)
  if not profile then
    gdbforge.print("ERROR: unknown profile " .. tostring(bad_profile) ..
      " (use baremetal, zephyr, or freertos)")
    return
  end

  local st = gdbforge.system("test -f " .. shell_quote(CFG))
  if st ~= 0 then
    gdbforge.print("ERROR: OpenOCD cfg not found: " .. CFG)
    return
  end
  st = gdbforge.system("test -f " .. shell_quote(TDESC))
  if st ~= 0 then
    gdbforge.print("ERROR: tdesc not found: " .. TDESC)
    return
  end
  gdbforge.print("R5 core: R" .. core)
  gdbforge.print("cfg: " .. CFG)
  gdbforge.print("tdesc: " .. TDESC)

  C.check_profile(profile, gdbforge.program())
  local rtos_args, rtos = C.openocd_rtos_args(profile)
  gdbforge.print(C.describe(profile, rtos))

  stop_openocd()

  gdbforge.print("starting openocd (Digilent HS2, R" .. core .. ") …")
  -- -c before -f so the cfg can read R5_CORE when selecting the GDB target; the RTOS -c
  -- after it, because "configure -rtos" needs the target to exist — still config stage,
  -- which is the only time OpenOCD accepts it.
  local argv = { OPENOCD, "-c", "set R5_CORE " .. core, "-f", CFG }
  for _, a in ipairs(rtos_args) do
    argv[#argv + 1] = a
  end
  gdbforge.spawn(unpack(argv))

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 20) then
    gdbforge.print("ERROR: OpenOCD did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")
  gdbforge.sleep(0.5)
  if not openocd_alive() then
    gdbforge.print("ERROR: openocd died after bind (zombie/crash) — check probe/USB, :b exec")
    return
  end

  gdbforge.open_buffer("gdb")
  gdbforge.gdb("set architecture arm")
  gdbforge.gdb("set tdesc filename " .. TDESC)
  C.gdb_setup(profile)
  gdbforge.gdb("target remote localhost:" .. PORT)
  gdbforge.gdb("monitor halt")
  tcm_ecc_init()
  gdbforge.gdb("load")
  gdbforge.gdb("set $pc = 0x0")
  gdbforge.gdb("break main")
  gdbforge.print("r5_baremetal_openocd_digilent done — Code leaf intact; :b exec for OpenOCD logs")
end

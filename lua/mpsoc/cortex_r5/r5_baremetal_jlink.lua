-- Cortex-R5 / J-Link bring-up.
-- Install: copy lua/mpsoc/cortex_r5 into .gdbforge/lua/ (keeps r5_target.xml beside the script)
-- Usage:   :lua r5_baremetal_jlink [baremetal|zephyr|freertos]
--
-- The profile picks the SEGGER RTOS plugin JLinkGDBServer loads, so RTOS threads reach GDB.
-- Read the Cortex-M caveat in r5_common.lua before trusting what those plugins report on
-- an ARMv7-R core; for Zephyr, r5_baremetal_openocd_digilent is the accurate path.
--
-- Env:
--   GDBFORGE_R5_CORE       RPU core: 0|1|R0|R1 (default 0 / R0)
--   GDBFORGE_JLINK_CHIP    J-Link chip prefix (default XCZU3CG) → CHIP_R5_N
--   GDBFORGE_JLINK         path to JLinkGDBServer
--   GDBFORGE_JLINK_DEVICE  full override e.g. XCZU3CG_R5_0 (else CHIP_R5_N;
--                          trailing _R5_N rewritten from R5_CORE when set)
--   GDBFORGE_JLINK_PORT    GDB listen port (default 2334)
--   GDBFORGE_TDESC         target description XML (default: script dir r5_target.xml)
--   GDBFORGE_R5_TCM_INIT   TCM banks to zero before load: atcm (default) | btcm | all | 0
--   GDBFORGE_JLINK_NORESET pass -noreset to JLinkGDBServer, so connecting keeps whatever
--                          PARK/psu_init left behind (clocks, resets, PLLs)
--   GDBFORGE_JLINK_RTOS    RTOS plugin for the zephyr/freertos profiles, when the stock
--                          GDBServer/RTOSPlugin_*.so beside JLinkGDBServer is not what you want
--   ZEPHYR_BASE            kernel tree, for GDB source paths under profile zephyr
--
-- Kills any existing JLinkGDBServer, then gdbforge.spawn (background — Code pane stays).
-- wait_port waits until JLink listens before target remote.
-- Optional: :b exec to watch JLink logs.
--
-- Nothing here initialises the PS. If the board is not booting an FSBL that does it, see
-- PARK below: a companion xsdb script the user runs by hand, outside gdbforge, before this.

local C = dofile(gdbforge.lua_dir() .. "/r5_common.lua")

local JLINK = os.getenv("GDBFORGE_JLINK")
  or "/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
local CHIP = os.getenv("GDBFORGE_JLINK_CHIP") or "XCZU3CG"
local PORT = os.getenv("GDBFORGE_JLINK_PORT") or "2334"
local SPEED = os.getenv("GDBFORGE_JLINK_SPEED") or "4000"
local TDESC = os.getenv("GDBFORGE_TDESC")
  or (gdbforge.lua_dir() .. "/r5_target.xml")

-- Mentioned in help(), never run from here: it drives xsdb, which needs hw_server to own the
-- JTAG cable, and JLinkGDBServer is holding that cable for as long as a session is open.
local PARK = "scripts/zynqmp-park-el3.sh"

-- Also only mentioned in help(): the RTT console bridge, bundled in this binary. It ends in
-- minicom, so it wants a terminal of its own, and it attaches to the server started here
-- rather than spawning one. Its own --help covers RTT itself in full.
local RTT = os.getenv("GDBFORGE_RTT_SH") or "gdbforge --run-script rtt.sh"

-- TCM as the R5 itself addresses it. 64KB per bank per core in split mode. ATCM alone by
-- default: an image that fits in it never touches BTCM, and BTCM is not always powered or
-- even mapped, in which case writing it fails and takes the probe's memory path down with
-- it — after which everything else, including inserting a breakpoint, reports
-- "Cannot access memory".
local ATCM = { name = "ATCM", base = 0x00000000 }
local BTCM = { name = "BTCM", base = 0x00020000 }
local TCM_BANK_SETS = { atcm = { ATCM }, btcm = { BTCM }, all = { ATCM, BTCM } }
local TCM_BANK_WORDS = 16384

-- Opt-in env flags: anything but unset/empty/0/no/false counts as on.
local function truthy(v)
  if v == nil then return false end
  v = v:lower()
  return not (v == "" or v == "0" or v == "no" or v == "false" or v == "off")
end

-- Parse GDBFORGE_R5_CORE → 0 or 1 (default 0). Accepts 0|1|R0|R1.
local function r5_core()
  local v = os.getenv("GDBFORGE_R5_CORE") or "0"
  v = tostring(v):gsub("^%s+", ""):gsub("%s+$", ""):upper():gsub("^R", "")
  if v == "0" or v == "1" then
    return tonumber(v)
  end
  return nil, v
end

local function jlink_device(core)
  local d = os.getenv("GDBFORGE_JLINK_DEVICE")
  if d == nil or d == "" then
    return CHIP .. "_R5_" .. core
  end
  if d:match("_R5_%d+$") then
    return (d:gsub("_R5_%d+$", "_R5_" .. core))
  end
  return d
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

-- Stop a leftover J-Link so -port is free before respawn (pidof + kill; no pkill).
local function stop_jlink()
  gdbforge.print("stopping existing JLinkGDBServer (if any) …")
  gdbforge.system(
    "pids=$(pidof JLinkGDBServer 2>/dev/null); " ..
    "if [ -n \"$pids\" ]; then " ..
    "kill $pids 2>/dev/null; sleep 0.3; " ..
    "kill -9 $pids 2>/dev/null; " ..
    "fi; sleep 0.2"
  )
end

-- True if a non-zombie JLinkGDBServer is still running.
local function jlink_alive()
  local st = gdbforge.system(
    "pids=$(pidof JLinkGDBServer 2>/dev/null); " ..
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
  local core = r5_core() or 0
  local device = jlink_device(core)
  gdbforge.print("r5_baremetal_jlink — kill old JLink, spawn, target remote, load, break main")
  gdbforge.print("Usage: :lua r5_baremetal_jlink [baremetal|zephyr|freertos]")
  gdbforge.print("")
  C.profile_help_lines("r5_baremetal_jlink", "jlink")
  gdbforge.print("")
  C.zephyr_help_lines()
  gdbforge.print("")
  gdbforge.print("What this assumes:")
  gdbforge.print("  FSBL already ran from boot.bin (board booted normally).")
  gdbforge.print("  This script only uploads your app ELF over J-Link (load + break main).")
  gdbforge.print("  It does NOT load FSBL or init DDR/clocks like Xilinx XSCT does.")
  gdbforge.print("  Full FSBL-then-app bring-up needs extra steps — not in this script.")
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
  gdbforge.print("  the target: a reset discards psu_init, and JLinkGDBServer resets on")
  gdbforge.print("  connect unless GDBFORGE_JLINK_NORESET=1 makes it start with -noreset.")
  gdbforge.print("  --clear-boot-mode to boot normally.")
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
  gdbforge.print("Console over SEGGER RTT, for when uart0 belongs to Linux on the APU. The R5")
  gdbforge.print("writes into a ring buffer in RAM and the probe pulls it out over the same")
  gdbforge.print("JTAG cable — no serial port, no contention for uart0. RTT is plain C with no")
  gdbforge.print("OS underneath, so this works in a bare metal app exactly as it does under")
  gdbforge.print("Zephyr; only how you turn it on differs:")
  gdbforge.print("  bare metal  compile SEGGER_RTT.c and your own SEGGER_RTT_Conf.h (the one")
  gdbforge.print("              shipped with Zephyr reads its sizes from Kconfig, so replace")
  gdbforge.print("              those six values with plain numbers). Add Syscalls_GCC.c and")
  gdbforge.print("              printf() works; otherwise SEGGER_RTT_WriteString(0, ...).")
  gdbforge.print("              Locking needs nothing: the ARMv7-R path just saves CPSR and")
  gdbforge.print("              does cpsid i.")
  gdbforge.print("  Zephyr      CONFIG_USE_SEGGER_RTT and CONFIG_RTT_CONSOLE, and the board")
  gdbforge.print("              Kconfig must select HAS_SEGGER_RTT — no Cortex-R SoC upstream")
  gdbforge.print("              does, and without it both are dropped from prj.conf in")
  gdbforge.print("              silence.")
  gdbforge.print("Either way emit \\r\\n yourself, or turn on Add Carriage Return in minicom:")
  gdbforge.print("nothing in the RTT path converts a bare \\n the way a UART console does.")
  gdbforge.print("Run the bridge by hand, in its own terminal, since it ends in minicom:")
  gdbforge.print("    " .. RTT)
  gdbforge.print("  It attaches to the RTT port (19021) of the server started here and")
  gdbforge.print("  bridges it to a pty with socat, so the JTAG session is left alone.")
  gdbforge.print("  Its --help is the long version of everything below.")
  gdbforge.print("")
  gdbforge.print("Then point the probe at the RTT control block. Auto-search will not find")
  gdbforge.print("it: the R5's TCM is outside the ranges J-Link scans for this device. The")
  gdbforge.print("address moves whenever .bss shifts, so read it back from your app's map")
  gdbforge.print("file — zephyr/zephyr.map here, whatever your link step emits elsewhere:")
  gdbforge.print([[    awk '$2=="_SEGGER_RTT" {print $1}' zephyr/zephyr.map]])
  gdbforge.print("  prints e.g. 0x0000000000008120 — then, at the gdb prompt:")
  gdbforge.print("    monitor exec SetRTTAddr 0x8120")
  gdbforge.print("    continue")
  gdbforge.print("  continue is not optional: RTT only flows while the core is executing.")
  gdbforge.print("  That address is the _SEGGER_RTT block — a 16-byte \"SEGGER RTT\" marker")
  gdbforge.print("  plus the ring descriptors (pBuffer, SizeOfBuffer, WrOff, RdOff) that the")
  gdbforge.print("  probe follows to the real up and down buffers. It writes RdOff back into")
  gdbforge.print("  target RAM after each read; that is the only thing the target ever learns")
  gdbforge.print("  about the host.")
  gdbforge.print("  The block lives in bss, so the TCM ECC init above has to have run first,")
  gdbforge.print("  or the very first RTT write aborts.")
  gdbforge.print("  A core parked in WFI cannot answer those reads: the probe halts it to")
  gdbforge.print("  poll, and gdb reports that halt as SIGTRAP. Bare metal loops usually spin")
  gdbforge.print("  and never see it; if yours does wfi, drop it while RTT is the console.")
  gdbforge.print("  Under Zephyr the idle thread always wfi's — select")
  gdbforge.print("  ARM_ON_ENTER_CPU_IDLE_HOOK and return false from z_arm_on_enter_cpu_idle().")
  gdbforge.print("")
  gdbforge.print("JTAG speed matters more for RTT than for anything else here. -speed is the")
  gdbforge.print("TCK clock in kHz, and load is a handful of big transfers while RTT is a")
  gdbforge.print("continuous background poll — thousands of small reads, so a bit error rate")
  gdbforge.print("too low to spoil a load still shows up as a console that stops and later")
  gdbforge.print("resumes. The usable ceiling is a property of the whole path (probe, ribbon,")
  gdbforge.print("and every TAP in the chain), not of the probe alone, so an entry-level")
  gdbforge.print("EDU Mini on a long ribbon can be marginal at 4000 and solid at 1000. The")
  gdbforge.print("cost is nothing worth measuring: a 60KB image still loads in well under a")
  gdbforge.print("second, and RTT needs a few hundred bytes per second.")
  gdbforge.print("Export it before starting gdbforge. The value is read once, when this")
  gdbforge.print("script loads, so exporting it afterwards in another terminal changes")
  gdbforge.print("nothing — that is the usual reason the spawn line still says 4000:")
  gdbforge.print("    export GDBFORGE_JLINK_SPEED=1000   # then restart gdbforge")
  gdbforge.print("  On a server that is already up, gdb changes it live without a restart:")
  gdbforge.print("    monitor speed 1000")
  gdbforge.print("  Use it to rule the link in or out: if lowering it stops the stalls it was")
  gdbforge.print("  signal integrity; if not, suspect WFI above before touching speed again.")
  gdbforge.print("")
  gdbforge.print("Setup (copy-paste into shell / script):")
  gdbforge.print("  export GDBFORGE_R5_CORE=0          # or 1 / R0 / R1 (default R0)")
  gdbforge.print("  export GDBFORGE_JLINK_CHIP=" .. CHIP)
  gdbforge.print("  export GDBFORGE_JLINK=" .. JLINK)
  gdbforge.print("  export GDBFORGE_JLINK_DEVICE=" .. device)
  gdbforge.print("  export GDBFORGE_JLINK_PORT=" .. PORT)
  gdbforge.print("  export GDBFORGE_TDESC=" .. TDESC)
  gdbforge.print("  export GDBFORGE_JLINK_NORESET=1    # keep psu_init state across connect")
  gdbforge.print("  export GDBFORGE_JLINK_SPEED=" .. SPEED .. "   # kHz; drop it if JTAG looks flaky")
  gdbforge.print("  export GDBFORGE_JLINK_RTOS=/path/to/RTOSPlugin.so  # zephyr/freertos profiles")
  gdbforge.print("  export GDBFORGE_RTT_SH=" .. RTT)
  gdbforge.print("After: :b exec for JLink logs")
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
  local device = jlink_device(core)

  -- tdesc is self-contained (no separate register.xml). Fail early if missing.
  local st = gdbforge.system("test -f " .. shell_quote(TDESC))
  if st ~= 0 then
    gdbforge.print("ERROR: tdesc not found: " .. TDESC)
    return
  end
  gdbforge.print("R5 core: R" .. core .. "  device: " .. device)
  gdbforge.print("tdesc: " .. TDESC)

  C.check_profile(profile, gdbforge.program())
  local rtos_args, rtos = C.jlink_rtos_args(profile, JLINK)
  gdbforge.print(C.describe(profile, rtos))

  stop_jlink()

  local noreset = truthy(os.getenv("GDBFORGE_JLINK_NORESET"))
  gdbforge.print("starting JLinkGDBServer …" .. (noreset and " (-noreset)" or ""))
  local argv = {
    JLINK,
    "-device", device,
    "-if", "JTAG",
    "-speed", SPEED,
    "-port", PORT,
  }
  if noreset then
    argv[#argv + 1] = "-noreset"
  end
  for _, a in ipairs(rtos_args) do
    argv[#argv + 1] = a
  end
  gdbforge.spawn(unpack(argv))

  gdbforge.print("waiting for port " .. PORT .. " …")
  if not gdbforge.wait_port(PORT, 15) then
    gdbforge.print("ERROR: JLink did not listen on :" .. PORT .. " — try :b exec")
    return
  end
  gdbforge.print("port " .. PORT .. " is open")
  gdbforge.sleep(0.5)
  if not jlink_alive() then
    gdbforge.print("ERROR: JLinkGDBServer died after bind (zombie/crash) — check probe/USB, :b exec")
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
  gdbforge.print("r5_baremetal_jlink done — Code leaf intact; :b exec for JLink logs")
end

#!/usr/bin/env bash
#
# zynqmp-park-el3.sh — leave a ZynqMP A53 core halted at EL3 with the PS initialised,
# then get out of the way so gdbforge (or any other bare-metal debugger) can attach.
#
# Everything here runs under the AMD/Xilinx tools: xsdb talks to hw_server, which owns the
# JTAG cable for the duration and is disconnected before this script exits. gdbforge is not
# involved and does not need to be — see --help for why this cannot be done from the
# debugger side.

# Sourcing this instead of running it used to take the caller's shell down with it. Every
# error path here is an "exit", which in a sourced file exits the shell that sourced it, and
# the "set -e" below would stay set in an interactive shell afterwards, so the next failing
# command anywhere — a grep that matched nothing — would close the terminal as well.
#
# Re-run as a child process instead, which is what sourcing it was meant to do anyway, and
# hand back its status with "return": the only way out of a sourced file that is not fatal.
# This has to come before "set -e" so a sourced shell never inherits it.
_park_self="${BASH_SOURCE[0]:-$0}"
_park_sourced=0
if [ -n "${BASH_SOURCE[0]:-}" ]; then
    [ "${BASH_SOURCE[0]}" != "$0" ] && _park_sourced=1
else
    # zsh (and anything else without BASH_SOURCE): $0 is the sourced file, and
    # ZSH_EVAL_CONTEXT ends in ":file" only when this is being sourced.
    case "${ZSH_EVAL_CONTEXT:-}" in *:file*) _park_sourced=1 ;; esac
fi
if [ "$_park_sourced" -eq 1 ]; then
    echo "[INFO] This script was sourced. Running it as a child process instead, so that" >&2
    echo "       an error cannot close this shell. Just run it next time:" >&2
    echo "           $_park_self ${*:---help}" >&2
    bash "$_park_self" "$@"
    _park_rc=$?
    unset _park_self _park_sourced
    return "$_park_rc"
fi
unset _park_self _park_sourced

set -e

SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"

PSU_INIT="${ZYNQMP_PSU_INIT:-}"
URL="${ZYNQMP_HW_SERVER_URL:-TCP:127.0.0.1:3121}"
CORE="0"
FULL_PSU_INIT=1
RESTORE_BOOT_MODE=0
CLEAR_ONLY=0
DRY_RUN=0
FORCE=0

usage() {
    cat <<'HELP'
zynqmp-park-el3.sh — hand a debugger an A53 core that is still at EL3

USAGE
  zynqmp-park-el3.sh -p /path/to/psu_init.tcl [options]
  zynqmp-park-el3.sh --clear-boot-mode            # undo: back to normal QSPI/SD boot
  zynqmp-park-el3.sh -p ... --dry-run             # print the xsdb Tcl, run nothing

────────────────────────────────────────────────────────────────────────────────
 WHY — the problem this solves
────────────────────────────────────────────────────────────────────────────────
  A bare-metal application built against the Xilinx standalone BSP with EL3 = 1
  and EL1_NONSECURE = 0 (see bspconfig.h) has exactly one entry path, and it is
  the EL3 one. boot.S reads currentEL and branches:

      mrs  x0, currentEL
      cmp  x0, #0xC          // EL3
      beq  InitEL3
      cmp  x0, #0x4          // EL1
      beq  InitEL1
      b    error             // <- everything else lands here
      ...
      error:  b  error       // spins forever, before _startup, before main

  So an app loaded onto a core that is at EL2 does not fail with a message. It
  loads, it runs, and it hangs in a two-instruction loop with no UART output,
  which looks exactly like a bad image or a broken probe.

  You cannot fix this after the fact. An ARM core cannot raise its own exception
  level: EL3 -> EL2 -> EL1 is a one-way trip taken by eret, and nothing the
  debugger writes puts it back. "set $pc" and "load" happen at whatever EL the
  core is already at.

  And a normally booted board has already made that trip. FSBL and ATF do run at
  EL3, but ATF hands U-Boot down to EL2, so by the time there is a U-Boot prompt
  to halt at, EL3 is gone. Measured on this board at the U-Boot prompt:

      (gdb) p/x $cpsr
      $1 = 0x200002c9         mode field 0b1001 = EL2h

  Halting during FSBL would technically work, but the window is milliseconds and
  FSBL is about to overwrite the memory you want to load into. The supported
  answer is not to boot at all.

────────────────────────────────────────────────────────────────────────────────
 HOW — what the script actually does
────────────────────────────────────────────────────────────────────────────────
  1. Refuses to start if openocd or JLinkGDBServer is running. One USB cable
     takes one debug server. Sharing it does not fail cleanly: the small
     transfers get through, so connect and the register writes all appear to
     work, and then a bulk transfer corrupts with "ftdi_read_data returned 69,
     expected 70" under a page of Tcl that blames everything except the cause.
     Close the gdbforge session first. --force skips the check.

  2. Writes 0x100 to CRL_APB.BOOT_MODE_USER (0xFF5E0200). Bit 8 is USE_ALT,
     which tells the BootROM to take ALT_BOOT_MODE from bits [15:12] instead of
     reading the mode pins, and 0 in that field means JTAG. The result is that
     no FSBL, no ATF and no U-Boot runs at all, and the cores come out of reset
     at EL3 and stay there.

     The bit survives a system reset by design, which is what makes step 3
     possible, and is cleared by power-on reset.

  3. rst -system, issued against the PSU target rather than a core, so it is a
     real system reset and the BootROM re-reads the boot mode it was just told.

  4. Sources your psu_init.tcl and runs psu_init — against the PSU target, not
     a core. This is the step that is easy to get wrong. Every mask_write in
     psu_init is a read-modify-write, and after a system reset every A53 sits in
     "APU Reset" where reads fail. Run against a core it dies part-way through
     init_ps with

         Cannot read memory if not stopped. Context Cortex-A53 #0 state: APU Reset

     leaving the PS half configured. The PSU target reaches the same registers
     over the DAP's AXI path, which needs no live core.

     psu_init is what supplies the clocks, PLLs and MIO that FSBL would have
     configured. Skip it and the core runs with no clocks and the UART prints
     nothing.

  5. rst -processor on the chosen A53, then stop. The core comes out of APU
     Reset at its reset vector, at EL3, with psu_init's clocks intact, and is
     parked there.

  6. Prints cpsr, disconnects, and leaves the board in JTAG boot mode so that
     whatever attaches next can reset the core again without a bootloader
     racing it.

────────────────────────────────────────────────────────────────────────────────
 THEN — attaching gdbforge
────────────────────────────────────────────────────────────────────────────────
  hw_server still holds the cable when this script returns. Release it, start
  your own probe, and in gdb:

      pkill hw_server
      # start openocd / JLinkGDBServer, attach gdbforge, then:
      monitor halt
      p/x $cpsr                 # mode nibble must be d (EL3h), not 9 (EL2h)
      load
      set $pc = &_boot          # or 0x0 for a DDR-linked app whose vectors are at 0
      break main
      continue

  Do not let the debugger reset the target. A reset discards psu_init and you
  are back to a board with no clocks. "monitor halt" is fine; "monitor reset"
  is not.

  $pc matters: the entry symbol is only at address 0 if the linker script put
  it there. An OCM-linked app has its vector table at 0xFFFC0000, so jumping to
  0 lands in whatever the BootROM left behind.

────────────────────────────────────────────────────────────────────────────────
 AFTERWARDS — the board will not boot from QSPI
────────────────────────────────────────────────────────────────────────────────
  USE_ALT is still set, so the board stays in JTAG boot mode. To anyone who
  power-cycles it and waits for a console it looks bricked. A power-on reset
  clears it on its own; a warm reset does not. To clear it deliberately:

      zynqmp-park-el3.sh --clear-boot-mode

────────────────────────────────────────────────────────────────────────────────
 OPTIONS
────────────────────────────────────────────────────────────────────────────────
  -p, --psu-init FILE   psu_init.tcl for this board. Required (or set
                        ZYNQMP_PSU_INIT). Vitis exports it with the platform,
                        usually <platform>/hw/psu_init.tcl; a Yocto build may
                        publish it beside the images under its own name.
                        It must match the silicon — a psu_init from a different
                        part programs the wrong PLL dividers.
  -u, --url URL         hw_server URL. Default TCP:127.0.0.1:3121, or
                        $ZYNQMP_HW_SERVER_URL.
  -c, --core N          A53 core to park. Default 0.
  --no-serdes           Run only the clock, MIO and peripheral tables, skipping
                        init_serdes and the serdes/resetout tables. Those bring
                        up the gigabit transceivers (USB3/PCIe/SATA/DisplayPort)
                        and on some boards that phase halts the target and takes
                        the JTAG session with it. A bare-metal app that wants a
                        PS UART and OCM needs none of it. Falls back to the full
                        psu_init if the tcl does not expose the tables.
  --restore-boot-mode   Clear USE_ALT before disconnecting, so the next reset
                        boots normally. Off by default, because it means the
                        next reset runs a bootloader over the core you just
                        parked.
  --clear-boot-mode     Do nothing but clear USE_ALT and exit. The undo.
  --dry-run             Print the generated Tcl to stdout and exit. Nothing
                        connects, nothing is written.
  -f, --force           Skip the openocd/JLinkGDBServer check. You will almost
                        certainly regret this.
  -h, --help            This text.

────────────────────────────────────────────────────────────────────────────────
 EXAMPLES
────────────────────────────────────────────────────────────────────────────────
  # park core 0 at EL3
  zynqmp-park-el3.sh -p ~/platform/hw/psu_init.tcl

  # a board whose serdes bring-up kills the JTAG session
  zynqmp-park-el3.sh -p ~/platform/hw/psu_init.tcl --no-serdes

  # remote hw_server, core 1
  zynqmp-park-el3.sh -p ./psu_init.tcl -u TCP:10.0.0.9:3121 -c 1

  # see exactly what would be sent to xsdb
  zynqmp-park-el3.sh -p ./psu_init.tcl --dry-run

  # give the board back
  zynqmp-park-el3.sh --clear-boot-mode

 REQUIREMENTS
  xsdb in PATH. Source the Vitis settings if it is not:
      source /tools/xilinx/Vitis/2024.2/settings64.sh

  The Vitis settings are the thing you source; this script you run. Sourcing it is
  caught and turned into a child process, because otherwise every error path in it
  would exit your shell instead of the script.
HELP
    exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--psu-init) PSU_INIT="$2"; shift 2 ;;
        -u|--url) URL="$2"; shift 2 ;;
        -c|--core) CORE="$2"; shift 2 ;;
        --no-serdes) FULL_PSU_INIT=0; shift ;;
        --restore-boot-mode) RESTORE_BOOT_MODE=1; shift ;;
        --clear-boot-mode) CLEAR_ONLY=1; shift ;;
        --dry-run) DRY_RUN=1; shift ;;
        -f|--force) FORCE=1; shift ;;
        -h|--help) usage 0 ;;
        *) echo "Error: unknown argument '$1'" >&2; usage 1 ;;
    esac
done

require_xsdb() {
    command -v xsdb >/dev/null 2>&1 || {
        echo "Error: xsdb is not in PATH. Source the Vitis settings first, e.g." >&2
        echo "       source /tools/xilinx/Vitis/2024.2/settings64.sh" >&2
        exit 1
    }
}

# One USB JTAG cable, one debug server. Matched on the process name rather than the command
# line, or any shell that merely mentions openocd trips it, and zombies are skipped since
# they have already released the USB device.
require_cable() {
    [[ "$FORCE" -eq 1 ]] && return 0
    local other="" names="" p st nm
    for p in $(pgrep -x 'openocd|JLinkGDBServer' 2>/dev/null || true); do
        st="$(ps -o stat= -p "$p" 2>/dev/null | tr -d ' ')"
        case "$st" in Z*|"") continue ;; esac
        nm="$(ps -o comm= -p "$p" 2>/dev/null)"
        other+="       $p $nm"$'\n'
        case " $names " in *" $nm "*) ;; *) names+="${names:+ }$nm" ;; esac
    done
    if [[ -n "$other" ]]; then
        echo "Error: another debug server is already holding the JTAG cable:" >&2
        printf '%s' "$other" >&2
        echo "       Close the gdbforge session, or:" >&2
        echo "           pkill ${names// / ; pkill }" >&2
        echo "       (--force to try anyway)" >&2
        exit 1
    fi
}

run_tcl() {
    if [[ "$DRY_RUN" -eq 1 ]]; then
        cat
        return 0
    fi
    local tcl
    tcl="$(mktemp -t zynqmp-el3-XXXXXX.tcl)"
    # An EXIT trap, not RETURN: set -e aborts the whole script when xsdb fails, and a RETURN
    # trap would never fire.
    trap 'rm -f "$tcl"' EXIT
    cat > "$tcl"
    xsdb "$tcl"
}

if [[ "$CLEAR_ONLY" -eq 1 ]]; then
    [[ "$DRY_RUN" -eq 1 ]] || require_xsdb
    [[ "$DRY_RUN" -eq 1 ]] || require_cable
    [[ "$DRY_RUN" -eq 1 ]] || \
        echo "[INFO] Clearing CRL_APB.BOOT_MODE_USER — the board will boot normally again."
    run_tcl <<EOF
connect -url {${URL}}
targets -set -nocase -filter {name =~ "*PSU*"}
mwr -force 0xFF5E0200 0x00000000
puts "BOOT_MODE_USER cleared. Power-cycle or reset to boot from the mode pins."
disconnect
EOF
    exit 0
fi

[[ -n "$PSU_INIT" ]] || {
    echo "Error: no psu_init.tcl. Pass -p FILE or set ZYNQMP_PSU_INIT." >&2
    echo "       Vitis exports it with the platform, usually <platform>/hw/psu_init.tcl." >&2
    echo "       $SCRIPT_NAME --help explains what it is needed for." >&2
    exit 1
}
[[ -f "$PSU_INIT" ]] || { echo "Error: psu_init.tcl not found -> $PSU_INIT" >&2; exit 1; }
PSU_INIT="$(cd "$(dirname "$PSU_INIT")" && pwd)/$(basename "$PSU_INIT")"

[[ "$CORE" =~ ^[0-3]$ ]] || { echo "Error: --core must be 0..3, got '$CORE'" >&2; exit 1; }

if [[ "$DRY_RUN" -eq 0 ]]; then
    require_xsdb
    require_cable
    echo "[INFO] hw_server -> $URL"
    echo "[INFO] psu_init  -> $PSU_INIT"
    echo "[INFO] core      -> Cortex-A53 #$CORE"
    [[ "$FULL_PSU_INIT" -eq 0 ]] && \
        echo "[INFO] --no-serdes: clocks, MIO and peripherals only; serdes and DDR skipped."
fi

if [[ "$RESTORE_BOOT_MODE" -eq 1 ]]; then
    BOOT_MODE_EPILOGUE='mwr -force 0xFF5E0200 0x00000000
puts "BOOT_MODE_USER cleared: the next reset boots normally."'
else
    BOOT_MODE_EPILOGUE='puts "Board left in JTAG boot mode, so a reset will not run a bootloader."
puts "Clear it with a power cycle, or: '"${SCRIPT_NAME}"' --clear-boot-mode"'
fi

run_tcl <<EOF
connect -url {${URL}}
# hw_server enumerates the debug targets asynchronously, so a "targets -set" issued
# immediately after connect finds an empty list and fails. Poll instead of sleeping a fixed
# time, which is both faster on a warm server and safer on a cold one.
set found 0
for {set i 0} {\$i < 30} {incr i} {
    if {![catch {targets -set -nocase -filter {name =~ "*PSU*"}}]} { set found 1 ; break }
    after 500
}
if {!\$found} {
    error "The PSU never appeared on the JTAG chain. Is the board powered?"
}

# USE_ALT (bit 8) tells the BootROM to take ALT_BOOT_MODE (bits 15:12) instead of the mode
# pins, and 0 there means JTAG: no FSBL, no ATF, no U-Boot, cores left at EL3. The bit
# survives the system reset below by design, and is cleared by power-on reset.
mwr -force 0xFF5E0200 0x00000100
# Against the PSU, not a core, so this is a full system reset and the BootROM re-reads the
# boot mode it was just given.
rst -system
after 3000

# The reset invalidated the target selection, so re-make it. psu_init must run against the
# PSU: every mask_write in it is a read-modify-write, and after a system reset the A53s sit
# in "APU Reset" where reads fail part-way through init_ps, leaving the PS half configured.
# The PSU target reaches the same registers over the DAP's AXI path, which needs no core.
targets -set -nocase -filter {name =~ "*PSU*"}
source {${PSU_INIT}}
if {${FULL_PSU_INIT}} {
    psu_init
} elseif {[info exists psu_pll_init_data]} {
    init_ps [subst {\$psu_mio_init_data \$psu_peripherals_pre_init_data \$psu_pll_init_data \$psu_clock_init_data }]
    init_ps [subst {\$psu_peripherals_init_data \$psu_resetin_init_data }]
    puts "psu_init: clocks, MIO and peripherals only (serdes and DDR skipped)"
} else {
    # An older psu_init.tcl that does not expose those tables; nothing to do but run it whole.
    puts "psu_init: this psu_init.tcl has no separable tables, running it in full"
    psu_init
}
after 1000

# Only now is there a core to talk to. "rst -processor" takes it out of APU Reset; it comes
# out at EL3 at its reset vector, and stop parks it there with psu_init's clocks intact.
targets -set -nocase -filter {name =~ "*A53*#${CORE}"}
rst -processor
after 500
# The reset catch usually leaves it halted already, and xsdb reports that as an error rather
# than a no-op: interactively it prints "Already stopped" and carries on, but in a script it
# aborts, on a board that is in exactly the state we wanted.
catch {stop}

puts ""
puts "Cortex-A53 #${CORE} — m (Bits \[4:0\]) must read d for EL3h:"
# rrd prints when xsdb drives a terminal but returns its text when running a script, which is
# why this comes out blank if the result is not printed explicitly.
set _cpsr [rrd cpsr]
if {\$_cpsr ne ""} { puts \$_cpsr }
puts ""
${BOOT_MODE_EPILOGUE}
puts "Parked at EL3, psu_init done. Releasing the JTAG session."
disconnect
EOF

[[ "$DRY_RUN" -eq 1 ]] && exit 0

cat <<EOF

[OK] Cortex-A53 #${CORE} is halted at EL3 with the clocks up. Check the cpsr above:
     the mode nibble must be d (EL3h). A 9 means EL2 and the app will not run.

     hw_server still owns the JTAG cable. Release it, start your probe, then in gdb:

         pkill hw_server
         monitor halt
         load
         set \$pc = &_boot
         break main
         continue

[WARNING] Do not let the debugger reset the target — a reset discards psu_init and
          you are back to a board with no clocks.
EOF

#!/usr/bin/env bash
#
# zynqmp-park-el3.sh — leave a ZynqMP A53 core halted at EL3 with the PS initialised,
# then get out of the way so gdbforge (or any other bare-metal debugger) can attach.
#
# Everything here runs under the AMD/Xilinx tools: xsdb talks to hw_server, which owns the
# JTAG cable for the duration. The hw_server is started here and stopped again before this
# script returns, so the cable is free for the debugger that comes next; one that was
# already running is left alone. gdbforge is not involved and does not need to be — see
# --help for why this cannot be done from the debugger side.
#
# The cable is either one hw_server drives itself (Digilent/Xilinx FTDI, the default) or a
# SEGGER J-Link bridged in over Xilinx Virtual Cable with --jtag jlink. Everything after
# the connect is identical either way.

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
JTAG="${ZYNQMP_JTAG:-digilent}"
XVC_URL="${ZYNQMP_XVC_URL:-TCP:127.0.0.1:2542}"
JLINK_SERIAL="${ZYNQMP_JLINK_SERIAL:-}"
XVCD_BIN="${ZYNQMP_JLINK_XVCD:-}"
HW_SERVER_BIN="${ZYNQMP_HW_SERVER_BIN:-}"

# Set by start_xvcd and start_hw_server only when this script is the one that started the
# server in question, so that cleanup never kills a server that was already there and
# belongs to someone else.
XVCD_PID=""
XVCD_LOG=""
HW_SERVER_PID=""
HW_SERVER_PGID=""
HW_SERVER_LOG=""
HW_SERVER_ORPHANED=0
TCL_FILE=""

usage() {
    cat <<'HELP'
zynqmp-park-el3.sh — hand a debugger an A53 core that is still at EL3

USAGE
  zynqmp-park-el3.sh -p /path/to/psu_init.tcl [options]
  zynqmp-park-el3.sh -p ... --jtag jlink          # over a SEGGER J-Link instead
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

  2. Starts a hw_server, unless something already answers on --url. xsdb is only
     a client; hw_server is the process that opens the cable, and "connect"
     fails outright if there is none. It is stopped again before this script
     prints its last line, so the probe is free for whatever attaches next —
     but only if this script started it, since a hw_server that was already
     there belongs to whoever started it.

  3. Writes 0x100 to CRL_APB.BOOT_MODE_USER (0xFF5E0200). Bit 8 is USE_ALT,
     which tells the BootROM to take ALT_BOOT_MODE from bits [15:12] instead of
     reading the mode pins, and 0 in that field means JTAG. The result is that
     no FSBL, no ATF and no U-Boot runs at all, and the cores come out of reset
     at EL3 and stay there.

     The bit survives a system reset by design, which is what makes step 4
     possible, and is cleared by power-on reset.

  4. rst -system, issued against the PSU target rather than a core, so it is a
     real system reset and the BootROM re-reads the boot mode it was just told.

  5. Sources your psu_init.tcl and runs psu_init — against the PSU target, not
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

  6. rst -processor on the chosen A53, then stop. The core comes out of APU
     Reset at its reset vector, at EL3, with psu_init's clocks intact, and is
     parked there.

  7. Prints cpsr, disconnects, stops the servers it started in step 2, and
     leaves the board in JTAG boot mode so that whatever attaches next can
     reset the core again without a bootloader racing it.

────────────────────────────────────────────────────────────────────────────────
 CABLES — Digilent directly, J-Link over XVC
────────────────────────────────────────────────────────────────────────────────
  hw_server only drives cables AMD ships a driver for: the Platform Cable USB II
  and the Digilent FTDI modules, which includes the JTAG-HS2/HS3/SMT2 dongles
  and the ones soldered onto the ZCU10x boards. A SEGGER J-Link is not one of
  them. Plug one in and it does not show up in "jtag targets" at all, however it
  is wired, because hw_server never looks for it.

  It can still be used, over Xilinx Virtual Cable. XVC is a small TCP protocol
  that says nothing more than "shift these bits through the TAP", and hw_server
  speaks it as a client, so anything that serves XVC becomes a cable it accepts.
  SEGGER ship exactly that server with the J-Link tools, as JLinkXVCDServer:

      xsdb -> hw_server -> XVC over TCP -> JLinkXVCDServer -> J-Link -> board

  Nothing above that line can tell the difference. The PSU and A53 targets
  enumerate, psu_init's DAP writes land, rst -system resets the chip. It is only
  slower, because every JTAG shift is now a TCP round trip, so expect psu_init
  to take seconds rather than milliseconds.

  --jtag jlink does three things: starts JLinkXVCDServer unless something is
  already serving --xvc-url, passes -xvc-url to the xsdb connect so hw_server
  opens that cable, and kills the server again on the way out — but only the one
  it started, so a server you left running stays running.

  Two things it cannot do for you. The J-Link has to be on the PS JTAG pins,
  since the PL-only chain has no DAP and therefore no A53 to park. And hw_server
  is still required: XVC replaces the cable driver, not the debug server. It is
  started for you the same way, and the two are stopped in the right order,
  hw_server first so it is not left talking to a socket that has gone away.

────────────────────────────────────────────────────────────────────────────────
 THEN — attaching gdbforge
────────────────────────────────────────────────────────────────────────────────
  Both servers this script started — hw_server, and the XVC one under
  --jtag jlink — are stopped before it returns, so the cable is free by the time
  you see the summary. Start your own probe and, in gdb:

      # start openocd / JLinkGDBServer, attach gdbforge, then:
      monitor halt
      p/x $cpsr                 # mode nibble must be d (EL3h), not 9 (EL2h)
      load
      set $pc = &_boot          # or 0x0 for a DDR-linked app whose vectors are at 0
      break main
      continue

  A server that was already running when this script started is a different
  matter: it was not this script's to stop, so it is still there holding the
  cable, and the summary says which ones and how to get rid of them:

      pkill hw_server ; pkill -9 JLinkXVCDServer

  SIGTERM is not enough for the XVC one — see --xvc-url below.

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
  -u, --url URL         Where hw_server is, or where to start one. Default
                        TCP:127.0.0.1:3121, or $ZYNQMP_HW_SERVER_URL. If
                        something already answers there it is used as-is and
                        left running; otherwise a hw_server is started on that
                        port and stopped again on exit, which is what hands the
                        cable to your debugger. Only a loopback address can be
                        started for you, since the server has to run on the
                        machine the cable is plugged into.
  -c, --core N          A53 core to park. Default 0.
  -j, --jtag KIND       Which cable to reach the board through. "digilent"
                        (default, or $ZYNQMP_JTAG) for anything hw_server drives
                        itself — the Digilent FTDI modules, Platform Cable USB
                        II — or "jlink" to bridge a SEGGER J-Link in over XVC.
                        See CABLES above.
  --xvc-url URL         Where the XVC server is, or where to start one.
                        Default TCP:127.0.0.1:2542, or $ZYNQMP_XVC_URL. If
                        something already answers there it is used as-is and
                        left running; otherwise a JLinkXVCDServer is started on
                        that port and killed on exit. Only a loopback address
                        can be started for you, since the server has to run on
                        the machine the probe is plugged into. It is killed with
                        SIGKILL, because while it is still opening the J-Link it
                        ignores SIGTERM and would be left behind holding the
                        probe. --jtag jlink only.
  --jlink-serial SN     Which J-Link, when more than one is plugged in. A serial
                        number or a nickname from JLinkConfig; passed to the XVC
                        server as -USB. --jtag jlink only, and ignored if the
                        server was already running.
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

  # the same thing over a J-Link
  zynqmp-park-el3.sh -p ~/platform/hw/psu_init.tcl --jtag jlink

  # one J-Link out of several, by serial number
  zynqmp-park-el3.sh -p ./psu_init.tcl --jtag jlink --jlink-serial 123456789

  # J-Link on another machine, with its XVC server already up over there
  zynqmp-park-el3.sh -p ./psu_init.tcl --jtag jlink --xvc-url TCP:10.0.0.9:2542

  # a board whose serdes bring-up kills the JTAG session
  zynqmp-park-el3.sh -p ~/platform/hw/psu_init.tcl --no-serdes

  # a hw_server already running on another machine, core 1
  zynqmp-park-el3.sh -p ./psu_init.tcl -u TCP:10.0.0.9:3121 -c 1

  # see exactly what would be sent to xsdb
  zynqmp-park-el3.sh -p ./psu_init.tcl --dry-run

  # give the board back
  zynqmp-park-el3.sh --clear-boot-mode

 REQUIREMENTS
  xsdb in PATH. Source the Vitis settings if it is not:
      source /tools/xilinx/Vitis/2024.2/settings64.sh

  hw_server comes with it and is taken from PATH, or from the directory xsdb
  itself was found in; $ZYNQMP_HW_SERVER_BIN overrides both with a full path.
  Nothing needs to be running before this script: it starts its own.

  For --jtag jlink, JLinkXVCDServer as well. It is looked for in PATH, then in
  /opt/SEGGER/JLink*/ and /opt/JLink*/, under either of the names SEGGER have
  used for it (JLinkXVCDServer, JLinkXVCDServerExe). $ZYNQMP_JLINK_XVCD
  overrides all of that with a full path.

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
        -j|--jtag) JTAG="$2"; shift 2 ;;
        --xvc-url) XVC_URL="$2"; shift 2 ;;
        --jlink-serial) JLINK_SERIAL="$2"; shift 2 ;;
        --no-serdes) FULL_PSU_INIT=0; shift ;;
        --restore-boot-mode) RESTORE_BOOT_MODE=1; shift ;;
        --clear-boot-mode) CLEAR_ONLY=1; shift ;;
        --dry-run) DRY_RUN=1; shift ;;
        -f|--force) FORCE=1; shift ;;
        -h|--help) usage 0 ;;
        *) echo "Error: unknown argument '$1'" >&2; usage 1 ;;
    esac
done

case "$JTAG" in
    digilent|jlink) ;;
    *) echo "Error: --jtag must be digilent or jlink, got '$JTAG'" >&2; exit 1 ;;
esac

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

# Both halves of a TCP:host:port, defaulted the way xsdb defaults them. Answers in
# TCP_HOST/TCP_PORT for the caller to copy out, since there are two such URLs here.
parse_tcp_url() {
    local u="$1" what="$2"
    [[ "$u" =~ ^[Tt][Cc][Pp]: ]] && u="${u:4}"
    TCP_HOST="${u%%:*}"
    TCP_PORT="${u##*:}"
    [[ -n "$TCP_HOST" ]] || TCP_HOST="127.0.0.1"
    [[ "$TCP_PORT" =~ ^[0-9]+$ ]] || {
        echo "Error: $what must be TCP:host:port, got '$1'" >&2
        exit 1
    }
}

# Is anything accepting connections there? bash's /dev/tcp rather than nc, which is not
# installed everywhere and comes in two incompatible flavours where it is. Under "timeout"
# because /dev/tcp has no connect deadline of its own, and a --xvc-url pointing at a host
# that silently drops packets would otherwise hang here for the kernel's SYN retries.
port_open() {
    timeout 2 bash -c 'exec 3<>"/dev/tcp/$1/$2"' _ "$1" "$2" >/dev/null 2>&1
}

# SEGGER have shipped this under both names, and the Linux packages do not put it in PATH.
find_xvcd() {
    local c
    if [[ -n "$XVCD_BIN" ]]; then
        [[ -x "$XVCD_BIN" ]] || { echo "Error: ZYNQMP_JLINK_XVCD is not executable -> $XVCD_BIN" >&2; exit 1; }
        return 0
    fi
    for c in JLinkXVCDServer JLinkXVCDServerExe; do
        if command -v "$c" >/dev/null 2>&1; then XVCD_BIN="$(command -v "$c")"; return 0; fi
    done
    # Newest install first, so an old J-Link package left lying around does not win.
    for c in $(ls -d /opt/SEGGER/JLink*/JLinkXVCDServer* /opt/JLink*/JLinkXVCDServer* \
                     /usr/lib/JLink*/JLinkXVCDServer* 2>/dev/null | sort -V -r); do
        if [[ -x "$c" && ! -d "$c" ]]; then XVCD_BIN="$c"; return 0; fi
    done
    return 1
}

# hw_server ships beside xsdb, which require_xsdb has already found, so fall back to
# looking there — both where xsdb was found and where it resolves to, since an install is
# often reached through a symlink into the Vitis bin directory.
find_hw_server() {
    local x c
    if [[ -n "$HW_SERVER_BIN" ]]; then
        [[ -x "$HW_SERVER_BIN" ]] || { echo "Error: ZYNQMP_HW_SERVER_BIN is not executable -> $HW_SERVER_BIN" >&2; exit 1; }
        return 0
    fi
    if command -v hw_server >/dev/null 2>&1; then HW_SERVER_BIN="$(command -v hw_server)"; return 0; fi
    x="$(command -v xsdb 2>/dev/null)" || return 1
    for c in "$(dirname "$x")/hw_server" "$(dirname "$(readlink -f "$x")")/hw_server"; do
        if [[ -x "$c" && ! -d "$c" ]]; then HW_SERVER_BIN="$c"; return 0; fi
    done
    return 1
}

# Is any of it still there? The whole group, not just the pid, since the process that holds
# the cable is two levels below the one that was started.
hw_server_alive() {
    if [[ -n "$HW_SERVER_PGID" ]]; then
        kill -0 -- "-$HW_SERVER_PGID" 2>/dev/null
    else
        kill -0 "$HW_SERVER_PID" 2>/dev/null
    fi
}

hw_server_signal() {
    if [[ -n "$HW_SERVER_PGID" ]]; then
        kill "-$1" -- "-$HW_SERVER_PGID" 2>/dev/null || true
    else
        kill "-$1" "$HW_SERVER_PID" 2>/dev/null || true
    fi
}

# The thing that actually owns the cable. xsdb is only a client of it, and "connect" fails
# outright if nothing is listening, so one has to exist before any of the Tcl below runs.
# Reuses a server that is already there, and in that case leaves it alone on the way out.
start_hw_server() {
    local i

    parse_tcp_url "$URL" "--url"
    HW_HOST="$TCP_HOST"
    HW_PORT="$TCP_PORT"

    if port_open "$HW_HOST" "$HW_PORT"; then
        echo "[INFO] hw_server -> $HW_HOST:$HW_PORT, already served (left running on exit)"
        return 0
    fi

    case "$HW_HOST" in
        127.0.0.1|localhost|::1) ;;
        *)
            echo "Error: nothing answers on $HW_HOST:$HW_PORT, and that is not this machine," >&2
            echo "       so hw_server cannot be started from here. On $HW_HOST, run:" >&2
            echo "           hw_server -s TCP::$HW_PORT" >&2
            exit 1 ;;
    esac

    find_hw_server || {
        echo "Error: hw_server is not in PATH and is not beside xsdb. It ships with Vitis;" >&2
        echo "       source the settings script, or point \$ZYNQMP_HW_SERVER_BIN at it." >&2
        exit 1
    }

    HW_SERVER_LOG="$(mktemp -t zynqmp-hw_server-XXXXXX.log)"
    echo "[INFO] hw_server -> $HW_SERVER_BIN, port $HW_PORT"
    # "set -m" so the server lands in a process group of its own, because what Vitis puts in
    # bin/hw_server is a shell wrapper that runs bin/loader, which runs
    # unwrapped/lnx64.o/hw_server: three live processes, no exec anywhere and no signal
    # forwarding. Kill the pid that $! hands back and only the outer wrapper dies — the real
    # server keeps running and keeps the cable, which is the exact failure this script
    # exists to prevent. A group can be signalled whole.
    set -m
    "$HW_SERVER_BIN" -s "TCP::$HW_PORT" >"$HW_SERVER_LOG" 2>&1 </dev/null &
    HW_SERVER_PID=$!
    set +m

    # Only ever signal the group if the child really did become its own leader. If it is
    # still in this script's group, "kill -- -PGID" would take the script down with it.
    HW_SERVER_PGID="$(ps -o pgid= -p "$HW_SERVER_PID" 2>/dev/null | tr -d ' ')"
    [[ "$HW_SERVER_PGID" == "$HW_SERVER_PID" ]] || HW_SERVER_PGID=""

    # Same reasoning as the XVC wait below: watch the port, not the process, and let the
    # server's own output be the error message if it never gets there. The port first, since
    # with the wrapper chain the pid that is being watched is not the one that opens it.
    for ((i = 0; i < 60; i++)); do
        if port_open "$HW_HOST" "$HW_PORT"; then
            echo "[INFO] hw_server -> serving on $HW_HOST:$HW_PORT (pid $HW_SERVER_PID)"
            return 0
        fi
        hw_server_alive || break
        sleep 0.25
    done

    echo "Error: hw_server never opened $HW_HOST:$HW_PORT. It said:" >&2
    sed 's/^/       /' "$HW_SERVER_LOG" >&2
    if grep -qi 'address already in use\|bind' "$HW_SERVER_LOG" 2>/dev/null; then
        echo "       The port is taken by something that is not accepting connections the way" >&2
        echo "       a hw_server does. Find it with: ss -ltnp sport = :$HW_PORT" >&2
    fi
    exit 1
}

# SIGTERM first, so it closes the cable on its own: a USB probe left half-claimed by a
# killed server often will not open again until it is replugged. SIGKILL only if it stays.
# The port is the thing actually waited on — it is the one piece of evidence that the
# process holding the cable, rather than just its wrapper, has gone.
stop_hw_server() {
    local i
    if [[ -n "$HW_SERVER_PID" ]]; then
        hw_server_signal TERM
        for ((i = 0; i < 40; i++)); do
            hw_server_alive || break
            port_open "$HW_HOST" "$HW_PORT" || break
            sleep 0.1
        done
        if hw_server_alive && port_open "$HW_HOST" "$HW_PORT"; then
            hw_server_signal KILL
        fi
        wait "$HW_SERVER_PID" 2>/dev/null || true
        if port_open "$HW_HOST" "$HW_PORT"; then
            HW_SERVER_ORPHANED=1
            echo "[WARNING] hw_server is still listening on $HW_HOST:$HW_PORT, so it still has the" >&2
            echo "          cable. Stop it with: pkill hw_server" >&2
        fi
        HW_SERVER_PID=""
        HW_SERVER_PGID=""
    fi
    [[ -n "$HW_SERVER_LOG" ]] && rm -f "$HW_SERVER_LOG"
    HW_SERVER_LOG=""
    return 0
}

# SIGKILL, not SIGTERM: while the server is still opening the J-Link it does not handle
# signals, so a TERM leaves it running and holding the probe.
stop_xvcd() {
    [[ -n "$XVCD_PID" ]] || return 0
    kill -9 "$XVCD_PID" 2>/dev/null || true
    wait "$XVCD_PID" 2>/dev/null || true
    [[ -n "$XVCD_LOG" ]] && rm -f "$XVCD_LOG"
    XVCD_PID=""
}

# hw_server before the XVC server it is a client of, so it lets go of the socket before the
# far end disappears underneath it.
cleanup() {
    [[ -n "$TCL_FILE" ]] && rm -f "$TCL_FILE"
    stop_hw_server
    stop_xvcd
}
# One EXIT trap for the lot, set before anything can create state. Not RETURN traps in the
# functions that own each thing: set -e aborts the whole script when xsdb fails, and a
# RETURN trap would never fire.
trap cleanup EXIT
# Ctrl-C has to arrive at the EXIT trap rather than go around it. The servers are started
# in process groups of their own so they can be signalled as one, which also means an
# interrupt sent to this script's group no longer reaches them: without this they would
# outlive the Ctrl-C, still holding the cable.
trap 'exit 130' INT
trap 'exit 143' TERM

# Put a J-Link on the far end of an XVC socket, so hw_server — which has no driver for one
# and never will — can use it as a cable. Reuses a server that is already there.
start_xvcd() {
    local p st held="" i

    parse_tcp_url "$XVC_URL" "--xvc-url"
    XVC_HOST="$TCP_HOST"
    XVC_PORT="$TCP_PORT"

    if port_open "$XVC_HOST" "$XVC_PORT"; then
        echo "[INFO] xvc       -> $XVC_HOST:$XVC_PORT, already served (left running on exit)"
        return 0
    fi

    case "$XVC_HOST" in
        127.0.0.1|localhost|::1) ;;
        *)
            echo "Error: nothing answers on $XVC_HOST:$XVC_PORT, and that is not this machine," >&2
            echo "       so the XVC server cannot be started from here. On $XVC_HOST, run:" >&2
            echo "           JLinkXVCDServer -Port $XVC_PORT" >&2
            exit 1 ;;
    esac

    # One J-Link, one thing talking to it. An XVC server on some other port has it open
    # already and ours would fail on the USB claim, several seconds later and less clearly.
    for p in $(pgrep -x 'JLinkXVCDServer|JLinkXVCDServerExe' 2>/dev/null || true); do
        st="$(ps -o stat= -p "$p" 2>/dev/null | tr -d ' ')"
        case "$st" in Z*|"") continue ;; esac
        held+="       $p $(ps -o comm= -p "$p" 2>/dev/null)"$'\n'
    done
    if [[ -n "$held" ]]; then
        echo "Error: an XVC server is running but not on $XVC_HOST:$XVC_PORT:" >&2
        printf '%s' "$held" >&2
        echo "       It is holding the J-Link, so a second one cannot open it. Either point" >&2
        echo "       --xvc-url at the port it is really serving, or: pkill -9 JLinkXVCDServer" >&2
        exit 1
    fi

    find_xvcd || {
        echo "Error: --jtag jlink needs JLinkXVCDServer, and it is not in PATH or under" >&2
        echo "       /opt/SEGGER/JLink*/ or /opt/JLink*/. It ships with the J-Link Software" >&2
        echo "       and Documentation Pack; point ZYNQMP_JLINK_XVCD at it if it lives" >&2
        echo "       somewhere else." >&2
        exit 1
    }

    XVCD_LOG="$(mktemp -t zynqmp-xvcd-XXXXXX.log)"
    echo "[INFO] xvc       -> $XVCD_BIN, port $XVC_PORT${JLINK_SERIAL:+, J-Link $JLINK_SERIAL}"
    if [[ -n "$JLINK_SERIAL" ]]; then
        "$XVCD_BIN" -Port "$XVC_PORT" -USB "$JLINK_SERIAL" >"$XVCD_LOG" 2>&1 </dev/null &
    else
        "$XVCD_BIN" -Port "$XVC_PORT" >"$XVCD_LOG" 2>&1 </dev/null &
    fi
    XVCD_PID=$!

    # Opening the J-Link takes a moment, and with no probe plugged in it never finishes:
    # the server sits in "Connecting to J-Link..." forever without opening the port. Wait
    # on the port rather than on the process, and give up with its own output as the
    # explanation.
    for ((i = 0; i < 60; i++)); do
        kill -0 "$XVCD_PID" 2>/dev/null || break
        if port_open "$XVC_HOST" "$XVC_PORT"; then
            echo "[INFO] xvc       -> serving on $XVC_HOST:$XVC_PORT (pid $XVCD_PID)"
            return 0
        fi
        sleep 0.25
    done

    echo "Error: the XVC server never opened $XVC_HOST:$XVC_PORT. It said:" >&2
    sed 's/^/       /' "$XVCD_LOG" >&2
    if grep -q 'Target voltage too low' "$XVCD_LOG" 2>/dev/null; then
        echo "       \"Target voltage too low\" is VTREF, the J-Link's reference voltage input:" >&2
        echo "       the probe is fine and the board is not driving it. Either the board is off," >&2
        echo "       or VTREF is simply not wired — it has to go to the PS JTAG bank supply," >&2
        echo "       1.8 V on ZynqMP, and wiring only TCK/TMS/TDI/TDO/GND is not enough." >&2
        echo "       Check it with:  JLinkExe -CommandFile <(printf 'ShowHWStatus\\nq\\n')" >&2
        echo "       Some J-Links can be told to assume a voltage instead, with the VTREF" >&2
        echo "       command, but the small ones (EDU Mini) answer \"does not support setting a" >&2
        echo "       fixed VTref\" and have to see the real thing." >&2
    elif ! grep -q 'S/N:' "$XVCD_LOG" 2>/dev/null; then
        echo "       It never got as far as reporting a serial number, so it found no probe at" >&2
        echo "       all: check the USB cable, and that JLinkExe can see it." >&2
    fi
    exit 1
}

run_tcl() {
    if [[ "$DRY_RUN" -eq 1 ]]; then
        cat
        return 0
    fi
    TCL_FILE="$(mktemp -t zynqmp-el3-XXXXXX.tcl)"
    cat > "$TCL_FILE"
    xsdb "$TCL_FILE"
}

# hw_server drives a Digilent cable itself; a J-Link it can only reach as an XVC client,
# which is one extra argument here and a server to start further down. Scanning the chain
# through XVC is slower too — every shift is a TCP round trip — so the wait for the PSU to
# turn up is longer, and a timeout there means something different.
if [[ "$JTAG" == "jlink" ]]; then
    CONNECT="connect -url {${URL}} -xvc-url {${XVC_URL}}"
    TARGET_RETRIES=120
    NO_PSU_MESSAGE="The PSU never appeared on the JTAG chain. Is the board powered, and is the J-Link on the PS JTAG pins rather than a PL-only chain?"
else
    CONNECT="connect -url {${URL}}"
    TARGET_RETRIES=30
    NO_PSU_MESSAGE="The PSU never appeared on the JTAG chain. Is the board powered?"
fi

if [[ "$CLEAR_ONLY" -eq 1 ]]; then
    if [[ "$DRY_RUN" -eq 0 ]]; then
        require_xsdb
        require_cable
        [[ "$JTAG" == "jlink" ]] && start_xvcd
        start_hw_server
        echo "[INFO] Clearing CRL_APB.BOOT_MODE_USER — the board will boot normally again."
    fi
    run_tcl <<EOF
${CONNECT}
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
    echo "[INFO] cable     -> $JTAG"
    # The XVC server first: it is the cable, and hw_server is only told to open it later,
    # but there is no reason to have a debug server up while the probe is still unclaimed.
    [[ "$JTAG" == "jlink" ]] && start_xvcd
    start_hw_server
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
${CONNECT}
# hw_server enumerates the debug targets asynchronously, so a "targets -set" issued
# immediately after connect finds an empty list and fails. Poll instead of sleeping a fixed
# time, which is both faster on a warm server and safer on a cold one.
set found 0
for {set i 0} {\$i < ${TARGET_RETRIES}} {incr i} {
    if {![catch {targets -set -nocase -filter {name =~ "*PSU*"}}]} { set found 1 ; break }
    after 500
}
if {!\$found} {
    error "${NO_PSU_MESSAGE}"
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

# Which servers were this script's to stop has to be noted before stopping them, since
# that is what clears the pids it is read from.
HW_SERVER_WAS_OURS=0
[[ -n "$HW_SERVER_PID" ]] && HW_SERVER_WAS_OURS=1
XVCD_WAS_OURS=0
[[ -n "$XVCD_PID" ]] && XVCD_WAS_OURS=1

# Hand the cable back before saying it is free, rather than leaving it to the EXIT trap:
# the text below is an instruction to start a probe, and it would otherwise be printed
# while the cable was still held.
stop_hw_server
stop_xvcd

# What is left holding the probe is whatever was already running when this started, plus
# anything the stop above failed to see off.
RELEASE=""
[[ "$HW_SERVER_WAS_OURS" -eq 0 || "$HW_SERVER_ORPHANED" -eq 1 ]] && RELEASE="pkill hw_server"
if [[ "$JTAG" == "jlink" && "$XVCD_WAS_OURS" -eq 0 ]]; then
    RELEASE="${RELEASE:+${RELEASE} ; }pkill -9 JLinkXVCDServer   # was already up, and still has the J-Link"
fi

if [[ -n "$RELEASE" ]]; then
    CABLE_NOTE="     A debug server is still holding the JTAG cable: one that was already running
     when this started and so was never this script's to stop, or one that would
     not stop when it was asked. Release it, start your probe, then in gdb:

         ${RELEASE}"
else
    CABLE_NOTE="     The servers this script started are stopped, so the JTAG cable is free. Start
     your probe, then in gdb:"
fi

cat <<EOF

[OK] Cortex-A53 #${CORE} is halted at EL3 with the clocks up. Check the cpsr above:
     the mode nibble must be d (EL3h). A 9 means EL2 and the app will not run.

${CABLE_NOTE}
         monitor halt
         load
         set \$pc = &_boot
         break main
         continue

[WARNING] Do not let the debugger reset the target — a reset discards psu_init and
          you are back to a board with no clocks.
EOF

#!/usr/bin/env bash
#
# rtt.sh — open the target's SEGGER RTT console as an ordinary serial terminal.
#
# RTT is not a serial port. It is a ring buffer in target RAM that the J-Link reads over
# the same JTAG cable the debugger is already using. The J-Link GDB server republishes it
# as a TCP socket; this script bridges that socket to a pty with socat so minicom, or any
# other terminal program, can open it like a normal port.
#
# It only ever attaches to a GDB server that is already running. It neither starts nor
# stops one by default, because the JTAG session is usually long-lived and shared with an
# open debugger. See --help for the whole picture, including why the RTT address has to be
# handed to the probe by hand.

set -u

usage() {
    cat <<'HELP'
rtt.sh — SEGGER RTT console as a normal terminal, over the debugger's JTAG cable

USAGE
  rtt.sh                        # attach to the running GDB server, open minicom
  rtt.sh --no-terminal          # just create the pty and print its path
  rtt.sh --map build/zephyr/zephyr.map
  rtt.sh --start-server         # spawn a GDB server too (only if none is running)

────────────────────────────────────────────────────────────────────────────────
 WHY — when you need this
────────────────────────────────────────────────────────────────────────────────
  On a ZynqMP the PS uart0 usually belongs to Linux on the APU. An R5 that also
  drives uart0 either fights Linux for it or has to be left mute. RTT sidesteps
  the question: the R5 writes into a ring buffer in its own RAM, and the probe
  pulls the bytes out over JTAG. No second cable, no contention, and it works
  before any driver is up.

  RTT is plain C with no OS underneath, so a bare-metal application uses it the
  same way a Zephyr one does. Only the enabling differs:

    bare metal   Compile SEGGER_RTT.c and a SEGGER_RTT_Conf.h of your own. The
                 copy shipped with Zephyr reads its sizes from Kconfig, so
                 replace those six values with plain numbers. Link
                 Syscalls_GCC.c and printf() works; otherwise call
                 SEGGER_RTT_WriteString(0, ...) directly. Locking needs no OS
                 support: the ARMv7-R path saves CPSR and does cpsid i.

    Zephyr       CONFIG_USE_SEGGER_RTT and CONFIG_RTT_CONSOLE, plus the board
                 Kconfig must "select HAS_SEGGER_RTT". No Cortex-R SoC upstream
                 does that, and without it both symbols are dropped from
                 prj.conf without a word of warning.

  Either way, emit \r\n yourself or turn on Add Carriage Return in the terminal.
  Nothing in the RTT path converts a bare \n the way a UART console driver does,
  so output arrives as a staircase otherwise.

────────────────────────────────────────────────────────────────────────────────
 THE ADDRESS — why the probe has to be told
────────────────────────────────────────────────────────────────────────────────
  The probe finds the buffers through the RTT control block, a struct named
  _SEGGER_RTT that starts with a 16-byte "SEGGER RTT" marker and carries the
  ring descriptors: pBuffer, SizeOfBuffer, WrOff and RdOff. The target advances
  WrOff as it writes; the probe reads the new bytes and writes RdOff back into
  target RAM. That write-back is the only thing the target ever learns about the
  host, which is also why output looks identical whether nobody is listening or
  the probe simply cannot reach the memory.

  J-Link can hunt for that marker on its own, but on a Cortex-R5 it will not
  find it: the R5's TCM is outside the ranges it scans for these devices. So
  read the address out of your link map and hand it over. It moves whenever .bss
  shifts, so read it again after a rebuild rather than memorising it:

      awk '$2=="_SEGGER_RTT" {print $1}' zephyr/zephyr.map

  prints something like 0x0000000000008120, and then at the gdb prompt:

      monitor exec SetRTTAddr 0x8120
      continue

  continue is not optional. RTT only flows while the core is executing. This
  script prints the exact line to paste, resolved from whichever map it found.

────────────────────────────────────────────────────────────────────────────────
 WHEN OUTPUT STALLS
────────────────────────────────────────────────────────────────────────────────
  A core parked in WFI cannot answer the probe's memory reads, so the probe
  halts it to poll, and a connected gdb reports that halt as SIGTRAP. Output
  stops, then resumes when you continue. Bare-metal loops usually spin and never
  see it. Zephyr's idle thread always wfi's: select ARM_ON_ENTER_CPU_IDLE_HOOK
  and return false from z_arm_on_enter_cpu_idle() to keep idle spinning.

  Failing that, suspect the JTAG clock. A load is a few big transfers, while RTT
  is a continuous poll of thousands of small reads, so an error rate too low to
  spoil a load still shows as a console that freezes and recovers. The usable
  ceiling belongs to the whole path — probe, ribbon, and every TAP in the chain
  — so an entry-level probe on a long ribbon can be marginal at 4000 kHz and
  solid at 1000. Lowering it costs nothing measurable here. Either export
  GDBFORGE_JLINK_SPEED=1000 before starting gdbforge, or change it live on a
  running server with "monitor speed 1000" at the gdb prompt.

  If the console stays empty from the very first line, and the control block
  lives in TCM, check that TCM was initialised before first use. .bss is NOBITS,
  so load never writes it, and on the R5 the first sub-word store into a granule
  with no valid ECC syndrome aborts.

────────────────────────────────────────────────────────────────────────────────
 OPTIONS
────────────────────────────────────────────────────────────────────────────────
  -m, --map FILE        Link map to read _SEGGER_RTT from. Default: the first of
                        ./zephyr/zephyr.map, ./zephyr.map, or the newest *.map
                        in the working directory.
      --pty PATH        Where to create the pty symlink. Default ~/rtt0.
      --port N          RTT port on the GDB server. Default 19021, which is what
                        the server uses even without -rtttelnetport.
      --gdb-port N      Port the GDB server listens on. Default 2334.
      --no-terminal     Set up the bridge, print the pty path, and wait. Use it
                        when you want to attach your own terminal program.
      --start-server    Spawn a GDB server if none is listening, instead of
                        refusing. Off by default so a shared JTAG session is
                        never disturbed.
      --device NAME     J-Link device for --start-server. Default XCZU3CG_R5_0.
  -h, --help            This text.

  Every default can also come from the environment: JLINK, DEVICE, GDB_PORT,
  RTT_PORT, PTY, MAP.

────────────────────────────────────────────────────────────────────────────────
 EXAMPLES
────────────────────────────────────────────────────────────────────────────────
  # usual case: a debugger session is already open on 2334
  gdbforge --run-script rtt.sh

  # a build tree somewhere else
  gdbforge --run-script rtt.sh --map ~/proj/build/zephyr/zephyr.map

  # bridge only, then attach something other than minicom
  gdbforge --run-script rtt.sh --no-terminal &
  picocom ~/rtt0

────────────────────────────────────────────────────────────────────────────────
 NOTES
────────────────────────────────────────────────────────────────────────────────
  The GDB server accepts exactly one RTT client at a time. Close any
  JLinkRTTClient or JLinkRTTViewer first; this script's socat counts as that one
  client while it runs.

  Hardware flow control must be off in the terminal. A pty never raises CTS, so
  with RTS/CTS enabled minicom sits silent and RTT looks dead. The bundled
  ~/.minirc.rtt profile, used when present, already turns it off.
HELP
}

JLINK=${JLINK:-/opt/JLink_Linux_V914a_x86_64}
DEVICE=${DEVICE:-XCZU3CG_R5_0}
GDB_PORT=${GDB_PORT:-2334}
RTT_PORT=${RTT_PORT:-19021}
PTY=${PTY:-$HOME/rtt0}
MAP=${MAP:-}
START_SERVER=${START_SERVER:-0}
TERMINAL=1

while [ $# -gt 0 ]; do
	case "$1" in
	-m | --map)
		MAP=$2
		shift 2
		;;
	--pty)
		PTY=$2
		shift 2
		;;
	--port)
		RTT_PORT=$2
		shift 2
		;;
	--gdb-port)
		GDB_PORT=$2
		shift 2
		;;
	--device)
		DEVICE=$2
		shift 2
		;;
	--no-terminal)
		TERMINAL=0
		shift
		;;
	--start-server)
		START_SERVER=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "rtt.sh: unknown option $1 (try --help)" >&2
		exit 1
		;;
	esac
done

for tool in socat ss; do
	command -v "$tool" >/dev/null || {
		echo "rtt.sh: $tool is not on PATH" >&2
		exit 1
	}
done

pids=()
cleanup() {
	for p in "${pids[@]:-}"; do
		[ -n "$p" ] && kill "$p" 2>/dev/null
	done
	# socat forks a child, and killing the PID recorded in $! leaves that child
	# alive holding the one connection the server allows. The next run would then
	# build a pty that never receives anything, so match on the command line.
	pkill -f "link=$PTY" 2>/dev/null
	for p in "${pids[@]:-}"; do
		[ -n "$p" ] && wait "$p" 2>/dev/null
	done
	rm -f "$PTY"
}
trap cleanup EXIT

# Clear an orphaned socat from a previous run that was SIGKILLed before cleanup
if pkill -f "link=$PTY" 2>/dev/null; then
	echo "Cleared a stale socat still holding port $RTT_PORT"
	sleep 1
fi

# The JTAG session is usually long-lived and shared with an open debugger, so
# attach to it and leave it alone. The GDB server listens for RTT on 19021 by
# default, even when it was started without -rtttelnetport.
if ss -tln 2>/dev/null | grep -q ":$GDB_PORT "; then
	echo "Attaching to the GDB server already on port $GDB_PORT - JTAG session left alone"
elif [ "$START_SERVER" = 1 ]; then
	echo "No GDB server on port $GDB_PORT, starting one for $DEVICE"
	"$JLINK/JLinkGDBServerCLExe" -device "$DEVICE" -if JTAG -speed 4000 \
		-port "$GDB_PORT" -rtttelnetport "$RTT_PORT" &
	pids+=($!)
	sleep 2
else
	echo "No GDB server on port $GDB_PORT. Start your usual one, or pass" >&2
	echo "--start-server to let this script spawn one." >&2
	exit 1
fi

# No pre-flight check of the RTT port: any probe connection is itself an RTT
# client, and the server allows only one - so probing causes exactly the failure
# it is meant to detect. If the port really is busy, socat passes the server's
# own error message straight through to the terminal.
#
# forever,retry: a J-Link reconnect drops the socket, and this keeps the pty alive
socat pty,raw,echo=0,link="$PTY" "tcp:localhost:$RTT_PORT,forever,retry=10" &
pids+=($!)

for _ in $(seq 20); do
	[ -e "$PTY" ] && break
	sleep 0.2
done
if [ ! -e "$PTY" ]; then
	echo "socat failed to create $PTY - is the GDB server up on port $RTT_PORT?" >&2
	exit 1
fi

echo "RTT console on $PTY"

# The control block address, for anyone driving gdb by hand. Resolved from the
# working directory, not from this script's location: gdbforge extracts bundled
# scripts to the user cache, so nothing useful is ever next to $0.
if [ -z "$MAP" ]; then
	for candidate in zephyr/zephyr.map zephyr.map; do
		[ -f "$candidate" ] && MAP=$candidate && break
	done
fi
if [ -z "$MAP" ]; then
	MAP=$(ls -t ./*.map 2>/dev/null | head -1)
fi
if [ -n "$MAP" ] && [ -f "$MAP" ]; then
	ADDR=$(awk '$2=="_SEGGER_RTT" {print $1}' "$MAP" | head -1)
	if [ -n "$ADDR" ]; then
		echo "From $MAP - run this in gdb, then 'continue':"
		printf '    monitor exec SetRTTAddr 0x%X\n' "$ADDR"
	else
		echo "No _SEGGER_RTT in $MAP - is RTT actually enabled in this build?" >&2
	fi
else
	echo "No link map found here; pass --map to get the SetRTTAddr line." >&2
fi

if [ "$TERMINAL" = 0 ]; then
	echo "Bridge is up. Ctrl-C to tear it down."
	wait
	exit 0
fi

# The ~/.minirc.rtt profile turns off hardware flow control. Without it minicom
# waits for a CTS that a pty never raises, and RTT looks broken.
if [ -f "$HOME/.minirc.rtt" ]; then
	minicom rtt
else
	echo "No ~/.minirc.rtt profile: remember Ctrl-A O to turn hardware flow control off."
	minicom -D "$PTY"
fi

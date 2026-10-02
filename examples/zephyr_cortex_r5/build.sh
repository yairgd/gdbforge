#!/usr/bin/env bash
set -euo pipefail

# -------------------------
# Paths / config
# -------------------------
# Resolved from the script's own location, not the caller's cwd, so every command works
# from anywhere in the tree.
APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
GDBFORGE_DIR="$(cd -- "$APP_DIR/../.." && pwd)"

# An existing Zephyr on this machine. This is the upstream convention and it is also the
# only thing west needs to find a workspace from outside it (`west topdir` falls back to
# deriving the topdir from ZEPHYR_BASE), so when it resolves, nothing is downloaded.
ZEPHYR_BASE="${ZEPHYR_BASE:-}"

# Remembers the tree that `init` settled on, so later builds need no environment at all.
# Gitignored — it holds a machine-specific absolute path.
ZEPHYR_BASE_FILE="$APP_DIR/.zephyr-base"

# Where `init` puts a workspace when there is no existing Zephyr to use.
ZEPHYR_WORKDIR="${ZEPHYR_WORKDIR:-$HOME/zephyrproject}"
ZEPHYR_VERSION="${ZEPHYR_VERSION:-v4.3.0}"
ZEPHYR_MANIFEST="${ZEPHYR_MANIFEST:-https://github.com/zephyrproject-rtos/zephyr}"
ZEPHYR_SDK_TOOLCHAIN="${ZEPHYR_SDK_TOOLCHAIN:-arm-zephyr-eabi}"

BOARD="${BOARD:-kv260_r5}"
CONSOLE="${CONSOLE:-jtag}"
BUILD_DIR="${BUILD_DIR:-$APP_DIR/build}"

# J-Link gdb server binary. r5_baremetal_jlink.lua reads it from the environment; only
# needed by `debug`, and only on the J-Link path.
GDBFORGE_JLINK="${GDBFORGE_JLINK:-}"

PRISTINE=0
ZEPHYR_OPT=""
CMAKE_ARGS=()

# -------------------------
# Helpers
# -------------------------
die() { echo "ERROR: $*" >&2; exit 1; }
note() { echo "==> $*"; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing command: $1"
}

# Markers that exist in a Zephyr repository root and nowhere else, so pointing this at a
# west topdir or at an unrelated directory fails here instead of inside cmake.
is_zephyr_tree() {
  [ -n "${1:-}" ] && [ -f "$1/Kconfig.zephyr" ] && [ -f "$1/west.yml" ]
}

# Pick the tree to build against, first match wins:
#   1. --zephyr <path>          explicit, this run only
#   2. $ZEPHYR_BASE             an existing install, the upstream convention
#   3. .zephyr-base             whatever `init` last settled on
#   4. $ZEPHYR_WORKDIR/zephyr   what `init --download` fetches
# Quiet, and returns 1 when nothing matches, so callers can offer to download instead.
find_zephyr() {
  local saved="" cand
  [ -f "$ZEPHYR_BASE_FILE" ] && saved="$(cat "$ZEPHYR_BASE_FILE")"

  for cand in "$ZEPHYR_OPT" "$ZEPHYR_BASE" "$saved" "$ZEPHYR_WORKDIR/zephyr"; do
    if is_zephyr_tree "$cand"; then
      ZEPHYR_BASE="$(readlink -f "$cand")"
      export ZEPHYR_BASE
      return 0
    fi
  done
  return 1
}

# For the commands that cannot do anything without a tree. Keep the two separate: die()
# exits outright, so a `find_zephyr || true` spelled with the fatal version would take the
# whole script down instead of falling through.
resolve_zephyr() {
  find_zephyr || die "No Zephyr tree found. Either point this at one you already have:
  $0 init --use /path/to/zephyrproject/zephyr
  (or export ZEPHYR_BASE=/path/to/zephyrproject/zephyr)
or let the script fetch one:
  $0 init --download"
}

# Locate the Zephyr SDK and export it, because cmake only finds one by itself if it was
# registered as a cmake package, and `west sdk install` is not the only way an SDK gets
# onto a machine. Quiet, returns 1 when there is nothing to find.
find_sdk() {
  local cand
  if [ -n "${ZEPHYR_SDK_INSTALL_DIR:-}" ] && [ -d "${ZEPHYR_SDK_INSTALL_DIR}" ]; then
    return 0
  fi
  if [ -d "$HOME/.cmake/packages/Zephyr-sdk" ]; then
    return 0
  fi
  for cand in "$HOME"/zephyr-sdk-* /opt/zephyr-sdk-*; do
    if [ -d "$cand/$ZEPHYR_SDK_TOOLCHAIN" ]; then
      ZEPHYR_SDK_INSTALL_DIR="$cand"
      export ZEPHYR_SDK_INSTALL_DIR
      return 0
    fi
  done
  return 1
}

# west is almost never on PATH: pip refuses a system-wide install on any distro that marks
# python as externally managed (PEP 668), so a Zephyr workspace keeps it in a virtualenv
# beside the manifest repo. Find that venv again rather than making the caller activate it.
activate_venv() {
  local topdir cand
  topdir="$(dirname "${ZEPHYR_BASE:-$ZEPHYR_WORKDIR/zephyr}")"

  for cand in "$topdir/.venv" "$topdir/venv" "$ZEPHYR_WORKDIR/.venv"; do
    if [ -f "$cand/bin/activate" ]; then
      # shellcheck disable=SC1091
      source "$cand/bin/activate"
      return 0
    fi
  done
  return 0
}

console_snippet() {
  case "$1" in
    jtag|rtt|jtag-console)    echo "jtag-console" ;;
    uart0|uart|uart0-console) echo "uart0-console" ;;
    none|board|default)       echo "" ;;
    *) die "Unknown console: $1 (want: jtag | uart0 | none)" ;;
  esac
}

elf_path() { echo "$BUILD_DIR/zephyr/zephyr.elf"; }

# J-Link cannot auto-find the RTT control block: its search only covers the usual
# Cortex-M SRAM windows, and the R5's TCM at 0x0 is not one of them. The address has to
# come from the map file and be handed to the probe by hand.
rtt_addr() {
  local map="$BUILD_DIR/zephyr/zephyr.map" raw
  [ -f "$map" ] || return 0
  raw="$(awk '$2=="_SEGGER_RTT" {print $1; exit}' "$map")"
  [ -n "$raw" ] || return 0
  # The map file writes it zero-padded to 64 bits; SetRTTAddr takes either, but the short
  # form is what anyone would type.
  printf '0x%x\n' "$raw"
}

# -------------------------
# init
# -------------------------
# Two ways in, and the only real difference is whether anything is downloaded:
#   init --use <path>   adopt a Zephyr already on this machine
#   init --download     fetch $ZEPHYR_VERSION into $ZEPHYR_WORKDIR
#   init                adopt one if it resolves, otherwise download
make_init() {
  local mode="auto"

  while [ $# -gt 0 ]; do
    case "$1" in
      --use)
        [ $# -ge 2 ] || die "--use needs a path to a Zephyr tree"
        ZEPHYR_OPT="$2"; mode="use"; shift 2 ;;
      --download|-D)
        mode="download"; shift ;;
      -h|--help)
        echo "Usage: $0 init [--use <zephyr-dir> | --download]"
        echo "  --use <dir>   adopt an existing Zephyr (the .../zephyrproject/zephyr dir)"
        echo "  --download    fetch $ZEPHYR_VERSION into $ZEPHYR_WORKDIR"
        echo "  (no option)   adopt an existing one if found, else download"
        exit 0 ;;
      *) die "Unknown option for init: $1 (try: $0 init --help)" ;;
    esac
  done

  if [ "$mode" = "use" ]; then
    is_zephyr_tree "$ZEPHYR_OPT" || die "Not a Zephyr repository: $ZEPHYR_OPT
Expected the manifest repo itself (the directory holding Kconfig.zephyr and west.yml),
e.g. ~/zephyrproject/zephyr — not the workspace above it."
  fi

  if [ "$mode" != "download" ] && find_zephyr; then
    note "Using existing Zephyr: $ZEPHYR_BASE (nothing downloaded)"
    # An adopted tree belongs to whoever set it up, so only look: a workspace that already
    # builds has a toolchain, and `west sdk install` on top of it would pull a second
    # multi-gigabyte copy for nothing.
    if find_sdk; then
      note "Zephyr SDK: ${ZEPHYR_SDK_INSTALL_DIR:-registered as a cmake package}"
    else
      note "WARNING: no Zephyr SDK found. If the build cannot find a compiler, either"
      note "         export ZEPHYR_SDK_INSTALL_DIR=/path/to/zephyr-sdk-<ver>, or install"
      note "         one with: west sdk install -t $ZEPHYR_SDK_TOOLCHAIN"
    fi
  else
    download_zephyr
    ZEPHYR_OPT="$ZEPHYR_WORKDIR/zephyr"
    resolve_zephyr
    install_sdk
  fi

  echo "$ZEPHYR_BASE" > "$ZEPHYR_BASE_FILE"
  note "Recorded in $ZEPHYR_BASE_FILE — '$0 build' now needs no environment"
}

download_zephyr() {
  need_cmd git
  need_cmd python3

  note "Fetching Zephyr $ZEPHYR_VERSION into $ZEPHYR_WORKDIR"
  mkdir -p "$ZEPHYR_WORKDIR"

  if [ ! -f "$ZEPHYR_WORKDIR/.venv/bin/activate" ]; then
    note "Creating virtualenv: $ZEPHYR_WORKDIR/.venv"
    python3 -m venv "$ZEPHYR_WORKDIR/.venv"
  fi
  # shellcheck disable=SC1091
  source "$ZEPHYR_WORKDIR/.venv/bin/activate"
  pip install --quiet --upgrade pip west

  # Both halves matter: an interrupted `west init` leaves .west/ behind with no manifest
  # repo under it, and taking that as "already initialized" turns the next run into a
  # confusing west update failure instead of simply finishing the init.
  if [ -d "$ZEPHYR_WORKDIR/.west" ] && [ -d "$ZEPHYR_WORKDIR/zephyr/.git" ]; then
    note "west workspace already initialized — skipping west init"
  else
    rm -rf "$ZEPHYR_WORKDIR/.west"
    west init -m "$ZEPHYR_MANIFEST" --mr "$ZEPHYR_VERSION" "$ZEPHYR_WORKDIR"
  fi

  cd "$ZEPHYR_WORKDIR"
  west update
  west packages pip --install
}

# Only ever called on the download path, where the workspace is this script's own.
install_sdk() {
  activate_venv
  need_cmd west

  if find_sdk; then
    note "Zephyr SDK already present: ${ZEPHYR_SDK_INSTALL_DIR:-registered as a cmake package}"
    return 0
  fi

  note "Installing Zephyr SDK toolchain: $ZEPHYR_SDK_TOOLCHAIN"
  west sdk install -t "$ZEPHYR_SDK_TOOLCHAIN"
}

# -------------------------
# build
# -------------------------
make_build() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -c|--console)
        [ $# -ge 2 ] || die "--console needs jtag, uart0 or none"
        CONSOLE="$2"; shift 2 ;;
      -b|--board)
        [ $# -ge 2 ] || die "--board needs a board name"
        BOARD="$2"; shift 2 ;;
      -d|--build-dir)
        [ $# -ge 2 ] || die "--build-dir needs a path"
        BUILD_DIR="$2"; shift 2 ;;
      --zephyr)
        [ $# -ge 2 ] || die "--zephyr needs a path"
        ZEPHYR_OPT="$2"; shift 2 ;;
      -p|--pristine)
        PRISTINE=1; shift ;;
      -O0|--no-opt)
        # -Og is the default (CONFIG_DEBUG_OPTIMIZATIONS). -O0 does not currently link:
        # it overflows the 64K TCM by about 20K, because text+rodata crosses 32K and the
        # MPU then rounds the whole ROM region up to 64K (see prj.conf). Kept because it
        # is the first thing anyone reaches for, and the failure is immediate and obvious.
        CMAKE_ARGS+=(-DCONFIG_NO_OPTIMIZATIONS=y); shift ;;
      -h|--help)
        echo "Usage: $0 build [-c jtag|uart0|none] [-b <board>] [-d <dir>] [-p] [-O0] [-- <cmake args>]"
        exit 0 ;;
      --) shift; CMAKE_ARGS+=("$@"); break ;;
      *) die "Unknown option for build: $1 (try: $0 build --help)" ;;
    esac
  done

  local snippet
  snippet="$(console_snippet "$CONSOLE")"

  resolve_zephyr
  activate_venv
  find_sdk || note "WARNING: no Zephyr SDK found — the build may not find a compiler"
  need_cmd west

  # Snippets and -D overrides go into the CMake cache and nothing takes them out again:
  # building -S uart0-console over a jtag-console dir keeps both and merges the two .conf
  # files, and one -DCONFIG_NO_OPTIMIZATIONS=y sticks to every later build. Stamp the whole
  # invocation and start from scratch whenever it differs.
  local stamp="$BUILD_DIR/.console"
  local key="$CONSOLE ${CMAKE_ARGS[*]-}"
  if [ -d "$BUILD_DIR" ] && [ "$(cat "$stamp" 2>/dev/null || true)" != "$key" ]; then
    note "Build dir was configured differently — forcing a pristine build"
    PRISTINE=1
  fi

  local args=(build -b "$BOARD" -d "$BUILD_DIR")
  if [ -n "$snippet" ]; then
    args+=(-S "$snippet")
  else
    note "No console snippet: the board default leaves the console on uart1"
  fi
  if [ "$PRISTINE" = "1" ]; then
    args+=(-p always)
  fi
  args+=("$APP_DIR")
  if [ "${#CMAKE_ARGS[@]}" -gt 0 ]; then
    args+=(-- "${CMAKE_ARGS[@]}")
  fi

  note "ZEPHYR_BASE=$ZEPHYR_BASE"
  note "west ${args[*]}"

  # Drop the stamp first: a build that dies partway leaves a cache nobody can describe, and
  # an absent stamp is what makes the next run go pristine instead of inheriting it.
  rm -f "$stamp"
  west "${args[@]}"
  echo "$key" > "$stamp"

  echo
  note "ELF: $(elf_path)"
  if [ "$snippet" = "jtag-console" ]; then
    local addr
    addr="$(rtt_addr)"
    [ -n "$addr" ] && note "RTT control block: $addr  (monitor exec SetRTTAddr $addr)"
  fi
}

# -------------------------
# debug
# -------------------------
# Copies the shipped Lua workflows into the example's .gdbforge/ and starts gdbforge on the
# ELF. The profile itself has to be typed in the TUI: there is no flag to autorun a script.
make_debug() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -d|--build-dir)
        [ $# -ge 2 ] || die "--build-dir needs a path"
        BUILD_DIR="$2"; shift 2 ;;
      -h|--help)
        echo "Usage: $0 debug [-d <dir>]"
        exit 0 ;;
      *) die "Unknown option for debug: $1 (try: $0 debug --help)" ;;
    esac
  done

  local elf
  elf="$(elf_path)"
  [ -f "$elf" ] || die "No $elf — run '$0 build' first"

  resolve_zephyr   # gdb needs it to find the kernel sources

  local lua_src="$GDBFORGE_DIR/lua/mpsoc/cortex_r5"
  [ -d "$lua_src" ] || die "Lua workflows not found at $lua_src"
  mkdir -p "$APP_DIR/.gdbforge/lua"
  cp -r "$lua_src" "$APP_DIR/.gdbforge/lua/"
  note "Lua workflows installed: $APP_DIR/.gdbforge/lua/cortex_r5"

  local bin
  bin="$(find_gdbforge)"

  if [ -n "$GDBFORGE_JLINK" ]; then
    export GDBFORGE_JLINK
  else
    note "GDBFORGE_JLINK is not set — needed for the J-Link profile, e.g."
    note "  export GDBFORGE_JLINK=/opt/JLink_Linux_V914a_x86_64/JLinkGDBServer"
  fi

  echo
  note "In gdbforge, bring the core up with one of:"
  echo "      :lua r5_baremetal_jlink zephyr              # J-Link"
  echo "      :lua r5_baremetal_openocd_digilent zephyr   # OpenOCD + JTAG-HS2"
  local addr
  # The symbol only exists in a jtag-console build, so its presence is the whole test.
  addr="$(rtt_addr)"
  if [ -n "$addr" ]; then
    echo "      monitor exec SetRTTAddr $addr                 # then: continue"
  fi
  echo

  cd "$APP_DIR"
  exec "$bin" "$elf"
}

find_gdbforge() {
  if [ -n "${GDBFORGE_BIN:-}" ]; then
    echo "$GDBFORGE_BIN"
  elif command -v gdbforge >/dev/null 2>&1; then
    command -v gdbforge
  elif [ -x "$GDBFORGE_DIR/bin/gdbforge" ]; then
    echo "$GDBFORGE_DIR/bin/gdbforge"
  else
    die "gdbforge not found. Build it (cd $GDBFORGE_DIR && task build) or set GDBFORGE_BIN."
  fi
}

# -------------------------
# rtt / info / clean
# -------------------------
# The RTT viewer runs in its own terminal, against the same probe.
make_rtt() {
  local addr
  addr="$(rtt_addr)"
  [ -n "$addr" ] || die "No _SEGGER_RTT in $BUILD_DIR/zephyr/zephyr.map — build with -c jtag first"
  echo "$addr"
}

make_info() {
  find_zephyr || true
  find_sdk || true

  local snippet
  snippet="$(console_snippet "$CONSOLE")"

  cat <<EOF
app dir       : $APP_DIR
gdbforge dir  : $GDBFORGE_DIR
ZEPHYR_BASE   : ${ZEPHYR_BASE:-<unresolved - run '$0 init'>}
recorded in   : $([ -f "$ZEPHYR_BASE_FILE" ] && echo "$ZEPHYR_BASE_FILE" || echo "<nothing recorded>")
zephyr sdk    : ${ZEPHYR_SDK_INSTALL_DIR:-<not found>}
board         : $BOARD
console       : $CONSOLE -> ${snippet:-<no snippet, board default on uart1>}
build dir     : $BUILD_DIR $([ -d "$BUILD_DIR" ] && echo "(console: $(cat "$BUILD_DIR/.console" 2>/dev/null || echo unknown))" || echo "(absent)")
elf           : $([ -f "$(elf_path)" ] && elf_path || echo "<not built>")
rtt address   : $(rtt_addr)
gdbforge      : $(find_gdbforge 2>/dev/null || echo "<not found>")
EOF
}

# Build output only. The Zephyr tree and the SDK are not this script's to delete; the
# recorded ZEPHYR_BASE survives too, so a clean build needs no re-init.
make_clean() {
  note "Removing build output under $APP_DIR"
  rm -rf "$APP_DIR"/build*/
  rm -rf "$BUILD_DIR"
  note "Done. Zephyr tree, SDK and .zephyr-base are untouched."
}

# -------------------------
# CLI parsing
# -------------------------
usage() {
  cat <<EOF
Usage: $0 <command> [options]

Commands:
  init        - Set up Zephyr. Either adopt one already on this machine or download one:
                  init --use <zephyr-dir>   adopt it (the dir with Kconfig.zephyr/west.yml)
                  init --download           fetch $ZEPHYR_VERSION into \$ZEPHYR_WORKDIR
                  init                      adopt if found, else download
                Installs the $ZEPHYR_SDK_TOOLCHAIN SDK and records the tree in
                .zephyr-base, so later commands need no environment.
  build       - Build the example: build [-c jtag|uart0|none] [-b <board>] [-d <dir>]
                             [-p|--pristine] [-O0] [--zephyr <dir>] [-- <cmake args>]
  debug       - Install the Lua workflows and start gdbforge on the ELF: debug [-d <dir>]
  rtt         - Print the _SEGGER_RTT address for 'monitor exec SetRTTAddr'
  info        - Show the resolved paths, board, console and build state
  clean       - Remove build output (never the Zephyr tree or the SDK)
  all         - init -> build

Consoles:
  jtag        - SEGGER RTT over the debug probe; leaves uart0 to Linux on the APU (default)
  uart0       - PS UART0 at 115200 8N1; needs the R5 to own uart0
  none        - no snippet; the board default puts the console on uart1

Env vars:
  ZEPHYR_BASE=/path/to/zephyrproject/zephyr   use an existing tree (beats .zephyr-base)
  ZEPHYR_WORKDIR=/path                        where init --download puts it
                                              (default: \$HOME/zephyrproject)
  ZEPHYR_VERSION=vX.Y.Z                       manifest revision (default: $ZEPHYR_VERSION)
  BOARD=name                                  (default: $BOARD)
  CONSOLE=jtag|uart0|none                     (default: jtag)
  BUILD_DIR=/path                             (default: <app dir>/build)
  GDBFORGE_BIN=/path/to/gdbforge              (default: PATH, then ../../bin/gdbforge)
  GDBFORGE_JLINK=/path/to/JLinkGDBServer      required by the J-Link profile

Examples:
  $0 init --use ~/zephyrproject/zephyr
  $0 init --download
  $0 build -c jtag
  $0 build -c uart0
  $0 debug
EOF
  exit 1
}

[ $# -ge 1 ] || usage
COMMAND="$1"
shift

case "$COMMAND" in
  init)  make_init "$@" ;;
  build) make_build "$@" ;;
  debug) make_debug "$@" ;;
  rtt)   make_rtt "$@" ;;
  info)  make_info "$@" ;;
  clean) make_clean "$@" ;;
  all)
    make_init
    make_build "$@"
    ;;
  -h|--help|help) usage ;;
  *) usage ;;
esac

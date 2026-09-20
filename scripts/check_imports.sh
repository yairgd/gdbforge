#!/usr/bin/env bash
# Fail if support packages import app packages, or app packages import each other.
# The UI framework now lives in the separate termforge module, so the old
# host-vs-app checks for internal/termui et al. are enforced in that repo.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
check() {
  local pkg="$1"
  local bad="$2"
  local hits
  hits=$(go list -f '{{range .Imports}}{{println .}}{{end}}' "$pkg" 2>/dev/null | grep -E "$bad" || true)
  if [[ -n "$hits" ]]; then
    echo "FORBIDDEN: $pkg imports:"
    echo "$hits" | sed 's/^/  /'
    fail=1
  fi
}

# check_only fails if pkg imports anything from this module outside $allowed.
check_only() {
  local pkg="$1"
  local allowed="$2"
  local hits
  hits=$(go list -f '{{range .Imports}}{{println .}}{{end}}' "$pkg" 2>/dev/null \
    | grep -E '^github\.com/yairgd/gdbforge/' | grep -vE "$allowed" || true)
  if [[ -n "$hits" ]]; then
    echo "FORBIDDEN: $pkg imports:"
    echo "$hits" | sed 's/^/  /'
    fail=1
  fi
}

# App packages (avoid matching module path github.com/.../gdbforge alone)
APPS='github.com/yairgd/gdbforge/internal/(gdb|dlv|mcp|gdbforge|app|broker|strategy|stock)(/|$)'

# Generic support packages must not reach into debugger-specific code.
check ./internal/luahost "$APPS"

# Backend clients are headless: they must not pull in the UI framework root.
check ./internal/dlv 'github.com/yairgd/termforge$'
check ./internal/gdb 'github.com/yairgd/termforge$'

# Widgets render state; they must not drive the debugger or MCP directly.
check ./internal/gdbforge/widgets 'github.com/yairgd/gdbforge/internal/(mcp|gdb)(/|$)'

# The binary is a thin entry point: main() wires argv to the application package
# and the --hold-inferior-tty helper, and holds no application logic itself.
check_only ./cmd/gdbforge 'github.com/yairgd/gdbforge/internal/(app|ttyhold)$'

# ttyhold is argv-level plumbing for the re-executed binary: no app state.
check_only ./internal/ttyhold '^$'

if [[ "$fail" -ne 0 ]]; then
  echo "import guardrails failed"
  exit 1
fi
echo "import guardrails OK"

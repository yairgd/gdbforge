#!/usr/bin/env bash
# Fail if host packages import app packages, or apps import each other.
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

# App packages (avoid matching module path github.com/.../gdbforge alone)
DBG='github.com/yairgd/gdbforge/internal/(gdb|dlv|mcp|gdbforge)(/|$)'
DEMO='github.com/yairgd/gdbforge/internal/demo(/|$)'
# Future trading names + demo: host must not import these either
APPS='github.com/yairgd/gdbforge/internal/(gdb|dlv|mcp|gdbforge|demo|broker|strategy|stock)(/|$)'

for pkg in \
  ./internal/termui \
  ./internal/platform \
  ./internal/commands \
  ./internal/collections \
  ./internal/ptyx \
  ./internal/luahost \
  ./internal/execcli \
  ./internal/core
do
  check "$pkg" "$APPS"
done

check ./internal/dlv 'github.com/yairgd/gdbforge/internal/termui'
check ./internal/gdbforge/widgets 'github.com/yairgd/gdbforge/internal/(mcp|gdb)(/|$)'
check ./internal/platform 'github.com/yairgd/gdbforge/internal/gdbforge'

# Cross-app: debugger must not import demo; demo must not import debugger
if [[ -d internal/demo ]]; then
  check ./internal/demo "$DBG"
  for pkg in $(go list ./internal/gdbforge/... ./internal/gdb/... ./internal/dlv/... ./internal/mcp/... 2>/dev/null); do
    check "$pkg" "$DEMO"
  done
fi
if [[ -d cmd/demo ]]; then
  check ./cmd/demo "$DBG"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "import guardrails failed"
  exit 1
fi
echo "import guardrails OK"

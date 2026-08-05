#!/usr/bin/env bash
# Serve gdbforge documentation locally.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
exec python3 -m mkdocs serve \
  --dev-addr "${MKDOCS_DEV_ADDR:-127.0.0.1:8765}" \
  "$@"

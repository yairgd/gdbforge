#!/usr/bin/env bash
# Serve gdbforge documentation locally (uses .venv-docs when present).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
VENV="${ROOT}/.venv-docs"
PY="python3"

if [[ -x "${VENV}/bin/python" ]]; then
  PY="${VENV}/bin/python"
elif ! "${PY}" -m mkdocs --version >/dev/null 2>&1; then
  echo "MkDocs not found. Run: ./docs/setup-docs-venv.sh" >&2
  exit 1
fi

exec "${PY}" -m mkdocs serve \
  --dev-addr "${MKDOCS_DEV_ADDR:-127.0.0.1:8765}" \
  "$@"

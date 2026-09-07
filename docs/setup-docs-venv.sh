#!/usr/bin/env bash
# Create or refresh the local MkDocs virtualenv (.venv-docs/).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENV="${ROOT}/.venv-docs"
PY="${PYTHON:-python3}"

if ! command -v "$PY" >/dev/null 2>&1; then
  echo "setup-docs-venv: ${PY} not found" >&2
  exit 1
fi

PY_VERSION="$("$PY" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')"

if [[ -d "$VENV" && "${FORCE_VENV:-}" == "1" ]]; then
  echo "Removing existing venv (--force)..."
  rm -rf "$VENV"
fi

if [[ -d "$VENV" && -f "${VENV}/pyvenv.cfg" ]]; then
  VENV_VERSION="$(grep '^version = ' "${VENV}/pyvenv.cfg" | awk '{print $3}' | cut -d. -f1-2)"
  if [[ -n "$VENV_VERSION" && "$VENV_VERSION" != "$PY_VERSION" ]]; then
    echo "Existing venv is Python ${VENV_VERSION}; requested ${PY_VERSION}." >&2
    echo "Re-run with: FORCE_VENV=1 PYTHON=${PY} $0" >&2
    exit 1
  fi
fi

if [[ ! -d "$VENV" ]]; then
  echo "Creating ${VENV} with $("$PY" --version)..."
  "$PY" -m venv "$VENV"
fi

"${VENV}/bin/python" -m pip install -U pip wheel
"${VENV}/bin/pip" install -r "${ROOT}/requirements-docs.txt"
echo "Docs venv ready: ${VENV}/bin/python"

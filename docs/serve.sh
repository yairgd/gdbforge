#!/usr/bin/env bash
# Serve xGDB documentation locally.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
exec go run ./cmd/docserve "$@"

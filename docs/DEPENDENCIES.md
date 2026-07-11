# Software Dependencies

This document describes **Go module dependencies** (third-party libraries) and **internal package dependencies** (how code in this repository imports other packages).

**Companion docs:** [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md) · [ARCHITECTURE.md](ARCHITECTURE.md)

---

## Table of contents

- [Quick answer: `internal/termui`](#quick-answer-internaltermui)
- [External dependencies (go.mod)](#external-dependencies-gomod)
- [Internal package graph](#internal-package-graph)
- [Per-package import rules](#per-package-import-rules)
- [Command binaries](#command-binaries)
- [Forbidden edges](#forbidden-edges)
- [Verifying imports locally](#verifying-imports-locally)

---

## Quick answer: `internal/termui`

**Yes — `internal/termui` depends only on the Go standard library and [tcell](https://github.com/gdamore/tcell).**

It does **not** import:

- `internal/core`
- `internal/gdb`
- any other third-party UI library

That is intentional: `termui` is the **standalone TUI framework** (layout, widgets, grid/canvas rendering, keyboard/mouse input via tcell). Domain logic and debugger backends stay outside it.

Wiring happens **above** `termui`:

| Layer | Role |
|-------|------|
| `internal/termui` | Generic terminal UI framework — window manager, layout, rendering |
| `internal/cgdb` | App state (`AppState`, modes) + debugger widgets (views) |
| `cmd/cgdb` | Application startup — services, models, event bus; composes `termui`, widgets, `core`, and `gdb` |

Services, models, and widgets are wired in **`cmd/cgdb`**. Data flows Service → Event Bus → Model → Widget; `termui` never imports services or models directly.

---

## External dependencies (go.mod)

Module path: `github.com/yairgd/cgdb-go`

| Dependency | Used by | Purpose |
|------------|---------|---------|
| [`github.com/gdamore/tcell/v2`](https://github.com/gdamore/tcell) | `internal/termui`, `internal/cgdb/widgets` | Terminal screen, input, styles |
| [`github.com/creack/pty`](https://github.com/creack/pty) | `internal/gdb` | Pseudo-terminal for GDB MI2 I/O |

**No third-party runtime deps:** `cmd/docserve` uses the Go standard library only.

**System tools (not Go modules):**

| Tool | Required by |
|------|-------------|
| `gdb` | `internal/gdb` at runtime |
| `go` (see `go.mod` for version) | build |

Run `go mod graph` or `go list -m all` for exact versions and transitive modules.

---

## Internal package graph

```mermaid
flowchart BT
    tcell["gdamore/tcell/v2"]
    pty["creack/pty"]

    termui["internal/termui"]
    cgdb_pkg["internal/cgdb"]
    widgets["internal/cgdb/widgets"]
    core["internal/core"]
    gdb["internal/gdb"]
    cgdb_cmd["cmd/cgdb"]
    docserve["cmd/docserve"]

    termui --> tcell
    widgets --> termui
    widgets --> core
    gdb --> core
    gdb --> pty

    cgdb_cmd --> termui
    cgdb_cmd --> cgdb_pkg
    cgdb_cmd --> widgets
    cgdb_cmd --> core
    cgdb_cmd --> gdb

    docserve --> stdlib["Go stdlib only"]

    gdb -.->|"must NOT import"| termui
    core -.->|"must NOT import"| termui
    termui -.->|"must NOT import"| core
```

---

## Per-package import rules

| Package | May import | Must not import |
|---------|------------|-----------------|
| **`internal/termui`** | stdlib, `tcell` | `core`, `gdb`, other UI libs |
| **`internal/core`** | stdlib | `termui`, `tcell`, `gdb` |
| **`internal/gdb`** | stdlib, `creack/pty`, `core` | `termui`, `tcell` |
| **`internal/cgdb/widgets`** | `termui`, `core`, stdlib | — (debugger panes) |
| **`internal/cgdb`** | stdlib only | `termui`, `gdb` — app state / modes |
| **`cmd/cgdb`** | `termui`, `cgdb`, `cgdb/widgets`, `core`, `gdb` | — (composition root) |
| **`cmd/docserve`** | stdlib only | — |

**Heuristic:** if code can be unit-tested without a terminal, it belongs in **`core`**, not in **`termui`**.

---

## Command binaries

| Binary | Path | Pulls in |
|--------|------|----------|
| **`cgdb`** | `cmd/cgdb` | `termui`, `cgdb/widgets`, `core`, `gdb`, `tcell`, `pty` |
| **`docserve`** | `cmd/docserve` | stdlib only |

Build all commands: `task build` or `go build ./cmd/...`.

---

## Forbidden edges

These import directions are **architectural violations** — do not add them:

```text
core   ──X──>  termui | tcell
gdb    ──X──>  termui | tcell
termui ──X──>  core | gdb
```

**Why:** debugger backends and domain logic must stay UI-agnostic so you can swap tcell for another renderer, add a web UI, or run GDB I/O in tests without a terminal.

**How data crosses the boundary:** channels and event structs in `core` (e.g. `GdbOutputMsg`), consumed by `cmd/cgdb` or widgets — not by importing `core` from `termui`.

---

## Verifying imports locally

Exact import lists change as code evolves. Regenerate them with:

```bash
# External modules
go list -m all

# Per-package imports
for pkg in ./internal/termui ./internal/core ./internal/gdb \
           ./internal/cgdb/widgets ./cmd/cgdb ./cmd/docserve; do
  echo "=== $pkg ==="
  go list -f '{{join .Imports "\n"}}' $pkg | sort -u
done
```

To check for forbidden imports:

```bash
go list -f '{{.ImportPath}} imports {{.Imports}}' ./internal/... ./cmd/...
```

---

## Related documentation

- [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md) — file layout and package responsibilities
- [ARCHITECTURE.md](ARCHITECTURE.md) — subsystems and data flow
- [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) — GDB backend and event bridge
- [OVERVIEW.md](OVERVIEW.md) — why tcell for cgdb-go

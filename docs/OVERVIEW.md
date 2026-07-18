# Project Overview

**cgdb-go** is a terminal-native debugger front-end inspired by [cgdb](https://github.com/cgdb/cgdb) but rebuilt from first principles in Go. It aims to combine the familiarity of a curses debugger UI with a modular architecture that supports multiple debugger backends and long-term extensibility.

**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) · [ROADMAP.md](ROADMAP.md) · [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)

---

## Table of contents

- [Vision](#vision)
- [Goals](#goals)
- [Motivation](#motivation)
- [Comparison to cgdb and gdb TUI](#comparison-to-cgdb-and-gdb-tui)
- [Target users](#target-users)
- [Non-goals (for now)](#non-goals-for-now)

---

## Vision

cgdb-go is **not a clone of Vim**. It is a **generic application framework** inspired by Vim's interaction model, with the GDB debugger as its first application.

Vim has a single data model (text buffers). This framework supports **multiple application-specific data models** — breakpoints, registers, and console output in a debugger; orders, portfolio, and charts in a trading app. The user still works with familiar concepts (`:buffer`, `:split`, `:vsplit`, `:tab`), but `:buffer` selects which **model** to display, not which file to open.

cgdb-go should feel like **cgdb for the 2020s**: a keyboard-driven debugger workspace in the terminal, with source views, breakpoints, registers, memory, and a GDB console — built as a **composable widget system over domain models**, not a monolithic ncurses application.

The long-term vision:

- A **Vim-inspired interaction framework** — normal mode, focus mode, and a `:` command line — applied to arbitrary application data, not only text files.
- A **single UI codebase** that adapts to GDB today, OpenOCD/JTAG tomorrow, and other domains (trading, monitoring, …) via application-specific models and services.
- **Scriptable automation** via Lua plugins for custom panes, workflows, and CI integration.
- **Efficient rendering** through an off-screen grid and future diff-based terminal updates.

cgdb-go is a **terminal debugger UI in Go**, inspired by [cgdb](https://github.com/cgdb/cgdb). The module path is `github.com/yairgd/cgdb-go`.

---

## Goals

| Goal | Description |
|------|-------------|
| **Model-driven UI** | Services → event bus → models → widgets; widgets never talk to services |
| **Modular UI** | Widgets, layout engine, and rendering backend are separate layers |
| **Backend agnostic** | `core.Session` interface; GDB is the first implementation; `:AI` shares the live session |
| **Terminal fidelity** | Unicode, box-drawing borders, ANSI-aware text rendering |
| **Low latency feel** | Off-screen grid; path to diff rendering to minimize I/O |
| **Contributor-friendly** | Clear package boundaries, documented architecture, browsable docs |
| **Familiar UX** | `:buffer` for models, split panes, tabs, Vim-style window commands |

---

## Motivation

### Why not just use cgdb?

[cgdb](https://github.com/cgdb/cgdb) is mature and widely used, but it carries decades of C/ncurses heritage:

- UI, layout, and GDB interaction are tightly coupled.
- Extending cgdb (custom panes, alternate backends) requires deep familiarity with its internals.
- Rendering is tied to ncurses; swapping backends or optimizing redraw is difficult.

cgdb-go treats these as **architectural constraints to avoid from day one**, not as bugs to patch later.

### Why not Bubble Tea / Lip Gloss?

Bubble Tea excels at application-level TUI with declarative models, but cgdb-go needs:

- Fine-grained **split-tree layout** with resizable panes and shared border drawing.
- A **replaceable framebuffer** (`Grid`) for diff rendering.
- Direct **tcell** access for mouse, focus, and low-level drawing control.

The cgdb-go stack (`internal/termui`) is intentionally lower-level than Bubble Tea.

### Why Go?

- Strong concurrency model for debugger I/O (PTY readers, async MI records).
- Single static binary deployment.
- Growing ecosystem for terminal UIs (`tcell`) and tooling.

---

## Comparison to cgdb and gdb TUI

| Aspect | **cgdb** | **gdb TUI** (`layout src`) | **cgdb-go** (target) |
|--------|----------|----------------------------|----------------------|
| **UI toolkit** | ncurses | readline + ANSI (limited layout) | tcell + custom Grid |
| **Layout** | Fixed panes, configurable | Single source + status; no splits | Recursive split tree in Workspace |
| **Command entry** | GDB console in dedicated window | Integrated in TUI | CmdLine (`:`) + per-pane input |
| **Extensibility** | Limited | GDB Python, no UI hooks | Planned Lua plugins + widget API |
| **Backends** | GDB only | GDB only | GDB first; OpenOCD/JTAG planned |
| **Rendering** | ncurses direct | Minimal | Widget → Canvas → Grid → tcell |
| **Language** | C | C (GDB internals) | Go |
| **Maturity** | Production | Production (inside GDB) | Early prototype |

### What cgdb-go preserves from cgdb

- Terminal-native workflow — no GUI dependency.
- Source + console + auxiliary views in one screen.
- Keyboard-first interaction with optional mouse support.

### What cgdb-go changes

- **Application models** instead of text buffers as the primary data unit.
- **Explicit widget tree** instead of implicit window list.
- **Layout engine** assigns geometry; widgets never set global coordinates.
- **Service → model → widget** data flow; widgets display models and never call services directly.
- **Event bus** decouples services from models and application dispatch.
- **Pluggable rendering** at the Grid → terminal boundary.

```mermaid
flowchart LR
    subgraph Legacy["cgdb / gdb TUI"]
        Monolith["Monolithic UI + GDB"]
    end

    subgraph cgdb-go["cgdb-go"]
        UI["termui"]
        Core["core"]
        GDB["gdb"]
        UI --> Core
        Core --> GDB
    end

    Legacy -.->|"tight coupling"| Monolith
```

---

## Target users

| User | Needs |
|------|-------|
| **Embedded developers** | GDB today; OpenOCD/JTAG later; register and memory views |
| **Kernel / systems hackers** | Multi-pane layout, scriptable workflows |
| **Daily C/C++ developers** | Fast terminal debugger with cgdb-like ergonomics |
| **Tool builders** | Clean APIs to embed or extend debugger panes |

---

## Non-goals (for now)

- Replacing GDB's own TUI inside the GDB project.
- GUI or web-based debugger (terminal-first).
- Shipping a production-ready 1.0 — the current codebase is an **architecture prototype**.
- Remote debugging transport (that belongs in backend layers, not the UI).

See [ROADMAP.md](ROADMAP.md) for phased delivery plans.

---

## Next steps

- Architecture deep dive: [ARCHITECTURE.md](ARCHITECTURE.md)
- UI internals: [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md)
- Onboarding: [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)

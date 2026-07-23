# gdbforge Documentation

**gdbforge** is a Vim-inspired terminal application framework built in Go on [tcell](https://github.com/gdamore/tcell). The **GDB debugger** is the first application on the framework. The UI lives in `internal/termui`; the debugger app is driven from `cmd/gdbforge`.

The project targets a **cgdb-like experience** with a cleaner **MVC** architecture: application models created at startup, widgets as views (callbacks + paint), controllers in `cmd/gdbforge`, a recursive split-tree workspace, a replaceable rendering pipeline, and services that do not depend on the UI layer. See [ARCHITECTURE.md — MVC](ARCHITECTURE.md#mvc-current).

Standalone diagram sources live under [`diagrams/`](diagrams/). Demo screencast: [`media/gdbforge-demo.mp4`](media/gdbforge-demo.mp4) (program: [`../examples/stack_demo.c`](../examples/stack_demo.c)).

---

## Documentation map

| Document | Audience | Use when |
|----------|----------|----------|
| **[README.md](README.md)** (this file) | Everyone | Index, quick links, how to view docs |
| **[OVERVIEW.md](OVERVIEW.md)** | Users, contributors | Vision, goals, comparison to cgdb / gdb TUI |
| **[ARCHITECTURE.md](ARCHITECTURE.md)** | Architects, reviewers | High-level subsystems and data flow |
| **[UI_ARCHITECTURE.md](UI_ARCHITECTURE.md)** | UI contributors | Widgets, canvas, grid, layout, focus |
| **[WINDOW_MANAGEMENT.md](WINDOW_MANAGEMENT.md)** | UI contributors | Splits, tabs, workspace, command line |
| **[RENDERING.md](RENDERING.md)** | Rendering contributors | Cells, borders, Unicode, diff rendering |
| **[INPUT.md](INPUT.md)** | UX contributors | Keyboard, mouse, modes, vim commands |
| **[COMMAND_SYSTEM.md](COMMAND_SYSTEM.md)** | UX / app contributors | Command tree, DSL, parser, tab completion |
| **[EXEC_SHELL.md](EXEC_SHELL.md)** | App / UX contributors | `:!` exec panes, rest-args, live prompt, Ctrl-O |
| **[DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) (GDB MI + dual PTY / IO console)** | Backend contributors | GDB MI2, `ptyx` mux, `:AI` / GdbMcpService, OpenOCD plans |
| **[PLUGINS.md](PLUGINS.md)** | Extensibility | Lua plans, feature panes, automation |
| **[DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md)** | New developers | Package layout and responsibilities |
| **[DEPENDENCIES.md](DEPENDENCIES.md)** | Architects, reviewers | Go modules and internal import rules |
| **[ROADMAP.md](ROADMAP.md)** | Planners | Current state, planned work, vision |
| **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** | Contributors | Onboarding, file walk order, pitfalls |
| **[HOSTING.md](HOSTING.md)** | DevOps | Local docs server, CI artifacts |

---

## Quick start

### Run the debugger prototype

```bash
go run ./cmd/gdbforge
```

Requires a terminal with UTF-8 support. Optional for `:AI`: set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`.

```bash
go run ./cmd/gdbforge -- ./hello
# then in cgdb:  :AI what breakpoints are set?
```

The prototype registers a split workspace, a functional `:` command line with **normal/command modes**, **Ctrl+W focus chords**, **`:!` exec panes**, **`:AI` in-app LLM**, and an event bus that dispatches domain events through `HandleCoreEvents`.

### View documentation in a browser

```bash
./docs/serve.sh
# or: go run ./cmd/docserve
```

Open [http://127.0.0.1:8765/](http://127.0.0.1:8765/) for this index with embedded Mermaid. **GitHub Pages:** see [HOSTING.md](HOSTING.md#github-pages).

| Page | URL |
|------|-----|
| Overview | [/doc/OVERVIEW.md](http://127.0.0.1:8765/doc/OVERVIEW.md) |
| Architecture | [/doc/ARCHITECTURE.md](http://127.0.0.1:8765/doc/ARCHITECTURE.md) |
| Developer guide | [/doc/DEVELOPER_GUIDE.md](http://127.0.0.1:8765/doc/DEVELOPER_GUIDE.md) |
| Diagrams index | [/diagrams](http://127.0.0.1:8765/diagrams) |

Markdown and Mermaid render in the browser via CDN (marked + mermaid). No build step required.

---

## Top-level UI layout

The root UI is a fixed three-band layout. **TabBar**, **Workspace**, and **CmdLine** are top-level components — they are **not** part of the split tree.

```text
+--------------------------------------------------+
| Tab1 | Tab2 | Tab3                               |
+--------------------------------------------------+
|                                                  |
|                 Workspace                        |
|         (recursive split tree)                   |
|                                                  |
+--------------------------------------------------+
| : command line                                   |
+--------------------------------------------------+
```

```mermaid
graph TB
    Root["Root"]
    TabBar["TabBar<br/>(fixed height)"]
    Workspace["Workspace<br/>(remaining area)"]
    CmdLine["CmdLine<br/>(fixed height)"]

    Root --> TabBar
    Root --> Workspace
    Root --> CmdLine
```

*Source: [`diagrams/top_level_ui.mermaid`](diagrams/top_level_ui.mermaid)*

See [WINDOW_MANAGEMENT.md](WINDOW_MANAGEMENT.md) for split trees, tabs, and the command line.

---

## Core design principles

1. Widgets should not know about screen coordinates.
2. Widgets draw only inside their assigned `Rect`.
3. `Canvas` provides local drawing coordinates.
4. The layout engine owns positioning.
5. The rendering backend should be replaceable.
6. Business logic lives in models; widgets display models and never talk to services directly.
7. TabBar, CmdLine, and Workspace are top-level UI components.
8. Only Workspace contains the recursive split tree.
9. Services and future debugger backends must not depend on the UI implementation.
10. Models are created at application startup; widgets are created when the user displays a model.

Full rationale: [ARCHITECTURE.md](ARCHITECTURE.md#design-principles).

---

## Repository layout (summary)

```text
gdbforge/
├── cmd/
│   ├── gdbforge/          # gdbforge debugger entry point
│   └── docserve/      # Documentation HTTP server
├── internal/
│   ├── termui/        # gdbforge terminal UI (primary)
│   ├── core/          # UI-agnostic logic (events, buffers)
│   ├── gdb/           # GDB MI2 client and parsing
│   ├── gdbforge/          # Layouts + debugger panes
└── docs/              # This documentation tree
```

Details: [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md).

---

## Implementation status (at a glance)

| Area | Status |
|------|--------|
| Split tree layout | Implemented |
| Per-pane focus status line | Implemented |
| Canvas / Grid / Cell borders | Implemented |
| GDB MI2 PTY client | Prototype |
| Root layout (TabBar + Workspace + CmdLine) | Partial |
| Interaction modes (Normal / Command) | Partial |
| Key-sequence trie (`Ctrl+W` focus) | Partial |
| Diff rendering | Planned |
| Focus mode / search mode | Planned |
| Lua plugins | Planned |

Full tracker: [ROADMAP.md](ROADMAP.md).

---

## Related links

- [CONTRIBUTING.md](../CONTRIBUTING.md) — contribution workflow
- [README.md (project root)](../README.md) — gdbforge overview

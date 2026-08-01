# Directory Structure

This document maps the **gdbforge** repository packages to their responsibilities.

**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) · [DEPENDENCIES.md](DEPENDENCIES.md) · [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)

---

## Table of contents

- [Repository tree](#repository-tree)
- [Command entry points](#command-entry-points)
- [internal/termui](#internaltermui)
- [internal/core](#internalcore)
- [internal/gdb](#internalgdb)
- [internal/dlv](#internaldlv)
- [internal/gdbforge](#internalgdbforge)
- [docs](#docs)
- [Dependency graph](#dependency-graph)
- [What belongs where](#what-belongs-where)

---

## FRAMEWORK vs APP

| Kind | Path | Notes |
|------|------|-------|
| FRAMEWORK | `internal/termui`, `platform`, `commands`, `collections`, `ptyx`, `luahost`, `core` | Reusable TUI / host |
| APP | `internal/gdb`, `dlv`, `mcp`, `internal/gdbforge/*`, `cmd/gdbforge` | Debugger-only |
| APP events | `internal/gdbforge/events` | `GdbOutputMsg`, `InferiorOutputMsg` (not in `core`) |
| APP state | `internal/gdbforge/debugstate` | Debugger fields formerly on `platform.AppState` |
| APP DTOs | `internal/gdbforge/models`, `parse`, `mitext` | Break/thread/stack types + MI parsers/string helpers |

Import guardrails: `task check-imports`.

---

## Repository tree

```text
gdbforge/
├── cmd/
│   ├── gdbforge/              # gdbforge debugger app
│   ├── docserve/          # Documentation HTTP server
│   └── dbug/              # (removed — was a dev helper)
├── internal/
│   ├── termui/            # TUI framework (tcell) ★
│   ├── commands/          # Command tree, parser, DSL, key bindings ★
│   ├── collections/       # Shared trie (keys + command children) ★
│   ├── platform/          # Buffer, Logger, AppContext ★
│   ├── luahost/           # Lua VM + framework API (APP wires gdb/dlv) ★
│   ├── ptyx/              # PTY sessions (FRAMEWORK) ★
│   ├── demo/               # Host showcase app (no debugger) ★
│   ├── gdbforge/              # Debugger app layer ★
│   │   ├── models/        # Break/thread/stack DTOs
│   │   ├── parse/         # MI parsers (not in mcp)
│   │   ├── mitext/        # MI string unescape / prompt tokens
│   │   ├── debugstate/    # Debugger AppState fields
│   │   ├── events/        # GdbOutputMsg / InferiorOutputMsg
│   │   ├── domain/
│   │   ├── layout/
│   │   ├── persist/
│   │   └── widgets/       # Debugger panes (no gdb/mcp imports)
│   ├── core/              # UI-agnostic domain + generic PTY/exec events ★
│   ├── gdb/               # GDB MI2 backend ★
│   ├── dlv/               # Delve CLI backend ★
│   ├── mcp/               # HTTP/MCP surface (thin; parsers elsewhere)
│   ├── playground/        # Experiments (not production)
│   └── tests/
├── docs/                  # gdbforge documentation ★
├── go.mod
├── Taskfile.yml
└── CONTRIBUTING.md
```

★ = primary gdbforge packages

---

## Command entry points

| Path | Binary | Purpose |
|------|--------|---------|
| `cmd/gdbforge/` | `gdbforge` | **gdbforge** debugger app (`package main`, split across files) |
| `cmd/demo/` | `demo` | Host showcase (gdbforge-like UI, basic commands; no GDB) |
| `cmd/docserve/main.go` | `docserve` | Serves `docs/` as HTML with Mermaid |

### `cmd/gdbforge` layout

| File | Responsibility |
|------|----------------|
| `main.go` | `main()` entry |
| `app.go` | `DebuggerApp`, models fields, `NewDebuggerApp`, `Close` |
| `setup.go` | `InitB` — chrome, mode handlers (`withGlobalKeys` for Ctrl-Z), Cmd `SetOnExecute` |
| `builtins.go` | Create models + views; wire intents; start GDB/IO bridges |
| `gdb_console.go` | GDB controller — Submit / MI paint / quit / suspend |
| `io_console.go` | Inferior PTY bridge + OutputWidget intents |
| `breakpoints.go` | Breakpoint model sync / toggle / delete / code Space; YAML restore |
| `debug_info.go` | Thread / call-stack / file-list view sync |
| `command_tree.go` | `ExapData` colon-command DSL |
| `keybindings.go` | `InitKeyBindings` (n/s/c, Space, …) |
| `actions.go` | Command actions (focus, split, quit, `:!` Exec, …) |
| `input.go` | Mode key handlers, global Ctrl-Z, mouse, resize, completion refresh |
| `completion.go` | CompletionMenu → CompletionView |
| `layout.go` | `:layout` apply / completions; wires `internal/gdbforge/layout` builders |
| `layout_behavior.go` | Per-layout normal-mode key policy (`HandleNormalKey`) |
| `focus.go` | App-private focus introspection (`focusedCode`, …); Tab stays generic |
| `workspace.go` | `Workspace` — pane marks, slot predicates; owns `TabWidget` |
| `workspace_policy.go` | Code/GDB/last activation, `FocusCode`, mark healing |
| `workspace_place.go` | `placeCodeInSlot`, logo slot, sticky-GDB swap / JumpBack |
| `workspace_layout.go` | `ApplyLayout` mounts layout `WidgetTree` onto Tab |
| `code_nav.go` | Thin Workspace delegates; `activeCodeWidget`; `sendGdbExec` |
| `events.go` | Debugger domain events (`BreakpointsChangedMsg`) |
| `stopped.go` | Stop handling; StopLocation; arm/trigger thread-stack refresh; MI thread/frame select |
| `debug_info.go` | `syncThreadViews` / `syncCallStackViews` / model setters |
| `lua.go` | ModeLua enter/leave; `:lua` / embedded script builtins |

Build all commands:

```bash
task build
# or: for d in cmd/*/; do go build -o bin/$(basename $d) ./$d; done
```

---

## internal/termui

**gdbforge TUI framework.** Depends on `tcell` only. App-specific widgets live in `internal/gdbforge/widgets`.

| File | Responsibility |
|------|----------------|
| `term_app.go` | Event loop, `AppApi`, `termui.Event` channel, widget list, grid buffers; `Suspend`/`Resume` (Ctrl-Z job control) |
| `event.go`, `command.go` | UI events (`SubmitMsg`, `CompletionMsg`), `CommandID` |
| `completion_bar.go` | Wildmenu chrome row (`ModeCompletion`); draw-only-when-active |
| `cmd_widget.go` | Global `:` command line (parser for Tab; `SetOnExecute` → app) |
| `history.go`, `autocomplete.go` | CmdLine history; legacy flat completer |
| `widget.go` | `Widget` interface |
| `node.go` | Split tree node types; `SetWidget` / `GetWidget` |
| `layout_tree.go` | Tree walks and ratio algorithms |
| `widget_tree.go` | Split/focus/geometry; `ReplaceFocusedWidget` |
| `tab.go` | Tab container over a per-tab `WidgetTree` |
| `canvas.go` | Local-coordinate drawing context |
| `grid.go` | Off-screen cell framebuffer |
| `cell.go` | Border edge composition |
| `rect.go` | Rectangle primitive |
| `utf.go` | UTF-8 / ANSI text drawing (`DrawANSIText`, `StripANSI`, width helpers) |
| `app_api.go` | `AppAPI` / `UIContext` interfaces |
| `base_widget.go` | Shared widget helpers: event channels, `PaneName`, key trie, default `DrawStatusLine` |
| `input_line.go` | Reusable readline editor (text, cursor, history, paste insert) |
| `console_pane.go` | Natural REPL transcript: scrollback + live/walking prompt + InputLine |
| `viewport.go` | Scroll window, follow-tail, selection/clipboard, optional ANSI + `OmitTail` |
| `viewport_word.go` | Double-click word / triple-click line select + copy |
| `viewport_clipboard.go` | Selection → CLIPBOARD + PRIMARY |
| `clipboard.go` | Middle-paste rising-edge + debounce; paste routing |
| `logger_widget.go` | Reusable log pane (`platform.Sink`, `:clear`, scroll bindings) |
| `status_line.go` | Per-pane status row helpers (`ClearStatusLine`, `PaintStatusBar`) |
| `named_widget.go` | Optional `WindowName()` hook for dynamic pane titles |

## internal/commands

**Hierarchical command tree** for colon commands, tab completion, and shared `CommandNode` types for key bindings.

| File | Responsibility |
|------|----------------|
| `command_node.go` | `CommandNode`, `CommandRegistry` — tree storage |
| `command_parser.go` | `CommandParser` — navigate tree, complete, execute |
| `dsl.go` | `Cmd`, `CmdRest`, `Group`, `Leaf`, `LeafRest` — declarative tree builder |
| `key_binding_gegistry.go` | `KeyBindingRegistry` — key chord → command |

See [COMMAND_SYSTEM.md](COMMAND_SYSTEM.md) for ownership (`CommandNode` = tree, `CommandRegistry` = owns, `CommandParser` = navigates).

## internal/gdbforge

**gdbforge application layer** — shared models, layout builders, and debugger views.

| Path | Responsibility |
|------|----------------|
| `models/breakpoints.go` | `BreakpointList` — shared BP model (GUI + MCP) |
| `models/threads.go` | `ThreadList` — stop snapshot |
| `models/callstack.go` | `CallStack` — frame snapshot |
| `persist/breakpoints.go` | `./.gdbforge/breakpoints.yaml` save/load |
| `domain/domain.go` | `DebugDomain` — peer-controller surface (AI now; future Lua) |
| `layout/` | Named workspace trees (`default`, `panels`, `classic`) — geometry only |
| `layout/default.go` | Multi-pane: Code/GDB left; IO / BP / Threads / Callstack right |
| `layout/panels.go` | Code/GDB left; IO over (Threads\|Callstack) over Breakpoints |
| `layout/classic.go` | Original cgdb: full-width Code over GDB |
| `widgets/code_widget.go` | Source view; Space → `OnBreakToggle`; gutters from `SetBreakInfos` |
| `widgets/breakpoint_widget.go` | `:b breakpoint`; `SetItems` + toggle/delete/activate intents |
| `widgets/thread_widget.go` | `:b threads`; `SetItems` from app model |
| `widgets/callstack_widget.go` | `:b callstack`; `SetItems` from app model |
| `widgets/output_widget.go` | `:b io`; paint inferior I/O; no PTY ownership |
| `widgets/about_widget.go` | Built-in About page (singleton via `:b about`) |
| `widgets/help_widget.go` | Viewport user manual (`:help` / `:b help`) |
| `widgets/logo_widget.go` | Startup splash in the code leaf until source loads |
| `widgets/gdb_widget.go` | GDB console view — ConsolePane + paint / `SetOn*` |
| `widgets/exec_widget.go` | Exec/shell console view — `SetOn*` + paint (`:!bash`) |

## internal/mcp

**In-process GDB tool service** for AI / MCP (same Session as the UI).

| File | Responsibility |
|------|----------------|
| `gdb_service.go` | `GdbMcpService` — `GdbCommand` under `WithWrite` + output capture |
| `tools.go` | LLM tool dispatch → `gdbforge/domain.DebugDomain` |
| `break_list.go` | Parse `-break-list` / pending BPs into `BreakInfo` |
| `thread_info.go` | Parse `-thread-info` into `ThreadInfo` |
| `stack_frames.go` | Parse `-stack-list-frames` into `StackFrame` |
| `agent.go` | `:AI` LLM loop (Anthropic / OpenAI) with domain tools + `gdb_command` |

## internal/ptyx

**PTY helpers** for GDB (MI), inferior I/O, and exec backends.

| File | Responsibility |
|------|----------------|
| `client.go` | `ptyx.Client` — process PTY: exclusive `WithWrite`, `Subscribe` fan-out, `Send` / `SetSize` / `Close` |
| `tty.go` | `ptyx.TTY` — bare master/slave for inferior stdin/stdout (`OpenTTY`, slave path for `-inferior-tty-set`) |

## internal/execcli

**External process PTY client** (Vim-style `:!` sessions) — thin wrapper over `ptyx`.

| File | Responsibility |
|------|----------------|
| `exec_client.go` | `ExecClient` embeds `*ptyx.Client`; argv + initial winsize |

See [EXEC_SHELL.md](EXEC_SHELL.md).

`cmd/gdbforge` wires `DebuggerApp` across `app.go`, `setup.go`, `input.go`, and related files (see table above).

```mermaid
flowchart TB
    TermApp --> Widget
    Widget --> Canvas
    Canvas --> Grid
    WidgetTree --> Node
```

---

## internal/core

**UI-agnostic domain logic.** No imports of `tcell` or other terminal packages.

Today this package holds shared primitives (`Buffer`, `Debugger` interface, backend event types). Explicit application models live in `internal/gdbforge/models` (breakpoints, threads, call stack).

| File | Responsibility |
|------|----------------|
| `events.go` | Backend events (`PtyOutputMsg`, `GdbOutputMsg` / `ExecOutputMsg` / `InferiorOutputMsg`, …) |
| `debugger.go` | `Debugger` / `Session` / `PTYWriter` — send, Subscribe, WithWrite |
| `buffer.go` | Line-oriented text storage — building block for text-oriented models |
| `viewport.go` | Scroll window over buffer |

**Rule:** if it can be tested without a terminal, it belongs here.

CmdLine helpers (`history`, `autocomplete`, command registry) live in **`termui`**, not `core`. UI domain events (`SubmitMsg`, `CommandID`) also live in **`termui`**; `core/events.go` holds debugger-backend event types (`GdbOutputMsg`, …).

---

## internal/gdb

**GDB MI2 backend.** Owns GDB PTY + inferior TTY; parses MI. Implements `core.Session`.

| File | Responsibility |
|------|----------------|
| `gdb_client.go` | `GDBClient` embeds `*ptyx.Client`, owns `*ptyx.TTY`; sends `-inferior-tty-set` at start |
| `mi.go` | MI string decode, field extraction, tab expansion |
| `mi_msg.go` | Batch line parser → structured `MiMsg` (helper / tests) |
| `mi_state.go` | Stream splitter: `PushRaw` → `MiUpdate` per complete MI line |

**Rule:** no imports from `termui`. GDB output → `GdbOutputMsg`; inferior stdio → `InferiorOutputMsg` (`EventInterrupt`).

Application orchestration for gdbforge lives in **`cmd/gdbforge`** (`DebuggerApp` embeds `termui.TermApp` and implements `HandleCoreEvents`).

---

## internal/dlv

**Delve interactive CLI backend** (peer of `internal/gdb`). Spawns `dlv exec --` on `ptyx`; implements `core.Session`.

| File | Responsibility |
|------|----------------|
| `client.go` | `Client` embeds `*ptyx.Client`; waits for `(dlv)` on startup |
| `input_state.go` | Stream splitter: `PushRaw` → `Update` (stops, prompts, `[Y/n]?`, BP notifies) |
| `confirm.go` | `ConfirmGate` for Delve yes/no prompts (suspended breakpoint after exit) |
| `complete.go` | Console Tab: command names + `funcs ^<prefix>` locspec completion |
| `parse.go` | Text parsers for `breakpoints` / `stack` / `goroutines` → MCP row types |

Selected with `gdbforge -g dlv`. See [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#delve-backend-peer-of-gdb).

---

## docs

| Path | Purpose |
|------|---------|
| `README.md` | Documentation index |
| `OVERVIEW.md` | Vision and comparison |
| `ARCHITECTURE.md` | High-level architecture |
| `PTY_ARCHITECTURE.md` | Dual PTY master/slave, `:b io`, external tty, Delve TCP |
| `UI_ARCHITECTURE.md` | Widget/canvas/grid details |
| `WINDOW_MANAGEMENT.md` | Splits, tabs, CmdLine |
| `RENDERING.md` | Grid, cells, diff rendering |
| `INPUT.md` | Keyboard, modes, commands |
| `COMMAND_SYSTEM.md` | Command tree, DSL, rest-args |
| `EXEC_SHELL.md` | `:!` exec panes, jump list |
| `DEBUGGER_INTEGRATION.md` | GDB MI / Delve details (see also PTY_ARCHITECTURE) |
| `PLUGINS.md` | Lua extensibility plans |
| `DIRECTORY_STRUCTURE.md` | This file |
| `DEPENDENCIES.md` | Go module + internal package rules |
| `ROADMAP.md` | Status and plans |
| `DEVELOPER_GUIDE.md` | Contributor onboarding |
| `HOSTING.md` | Docs server |
| `diagrams/*.mermaid` | Standalone diagram sources |
| `www/` | Browser viewer assets |
| `serve.sh` | Launch docs server |

---

## Dependency graph

Full detail: **[DEPENDENCIES.md](DEPENDENCIES.md)**.

```mermaid
flowchart BT
    tcell["gdamore/tcell"]
    termui["internal/termui"]
    gdbforge_pkg["internal/gdbforge"]
    widgets["internal/gdbforge/widgets"]
    core["internal/core"]
    gdb["internal/gdb"]
    gdbforge_cmd["cmd/gdbforge"]

    termui --> tcell
    widgets --> termui
    widgets --> core
    gdb --> core

    gdbforge_cmd --> termui
    gdbforge_cmd --> gdbforge_pkg
    gdbforge_cmd --> widgets
    gdbforge_cmd --> core
    gdbforge_cmd --> gdb

    gdb -.->|"must NOT import"| termui
    core -.->|"must NOT import"| termui
    termui -.->|"must NOT import"| core
```

---

## What belongs where

| Question | Package |
|----------|---------|
| Application model (domain state)? | `internal/gdbforge/models` |
| Peer control surface (AI / Lua)? | `internal/gdbforge/domain` (+ `cmd/gdbforge/debug_domain.go` impl) |
| Service (external I/O)? | `gdb` or future backend packages |
| Split pane layout / window manager? | `termui` |
| Widget (view of a model)? | `internal/gdbforge/widgets` or `termui` |
| GDB MI parsing? | `gdb` |
| Scrollable text storage primitive? | `core` |
| Key binding in normal mode? | `cmd/gdbforge/keybindings.go` + `input.go` |
| Interaction mode state? | `platform.AppState` via `TermApp` |
| Spawn/debug external process? | `gdb` (or future backend service) |
| `:buffer` / model registry? | App startup + `HandleCoreEvents` dispatch |
| Vim `:` command registry? | `internal/commands` + `cmd/gdbforge/command_tree.go` |
| Draw box borders? | `termui` Grid/Cell |
| Compose services + models + UI? | `cmd/gdbforge/setup.go` |

When unsure, ask: **"Can this be unit-tested without a terminal?"** — if yes, prefer `core` or a dedicated model package over `termui`.

---

## Related documentation

- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) — file walk order
- [ARCHITECTURE.md](ARCHITECTURE.md) — subsystem overview

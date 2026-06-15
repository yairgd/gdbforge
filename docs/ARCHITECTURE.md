# Architecture Overview

This document describes the high-level architecture of **NewCGDB**: subsystems, boundaries, data flow, and the design principles that govern implementation decisions.

**Companion docs:** [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) · [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md)

---

## Table of contents

- [System context](#system-context)
- [High-level architecture](#high-level-architecture)
- [Main subsystems](#main-subsystems)
- [Data flow](#data-flow)
- [Design principles](#design-principles)
- [Layer responsibilities](#layer-responsibilities)
- [Core events layer](#core-events-layer)
- [Current vs target architecture](#current-vs-target-architecture)

---

## System context

NewCGDB runs as a terminal application. It owns the UI event loop, renders into an off-screen grid, and communicates with debugger backends through abstract interfaces. The first backend is **GDB over MI2** via a pseudo-terminal.

```mermaid
flowchart LR
    User["Developer"]
    Term["Terminal"]
    NewCGDB["NewCGDB · TermApp"]
    GDB["GDB MI2"]
    Target["Debug target"]

    User --> Term
    Term <--> NewCGDB
    NewCGDB <--> GDB
    GDB <--> Target
```

---

## High-level architecture

```mermaid
flowchart TB
    subgraph Presentation["Presentation · internal/termui"]
        TermApp["TermApp"]
        RootLayout["Root: TabBar / Workspace / CmdLine"]
        SplitTree["Split tree · Layout / WidgetTree"]
        Widgets["Widgets: Code, GDB, Cmd, …"]
        Render["Canvas → Grid → tcell"]
    end

    subgraph Application["Application · internal/app"]
        State["AppState"]
        Modes["Modes"]
        Handlers["Event handlers"]
    end

    subgraph Domain["Domain · internal/core"]
        Events["Event bus"]
        Buffer["Buffer / Viewport"]
        History["History / Autocomplete"]
        DebuggerIF["Debugger interface"]
    end

    subgraph Infrastructure["Infrastructure · internal/gdb"]
        Client["GDBClient · PTY"]
        MI["MI parser · MiMsg"]
    end

    TermApp --> RootLayout --> SplitTree --> Widgets --> Render
    Widgets --> Application
    Application --> Domain
    Domain --> Infrastructure
```

*Source: [`diagrams/module_boundaries.mermaid`](diagrams/module_boundaries.mermaid)*

---

## Main subsystems

| Subsystem | Package | Responsibility |
|-----------|---------|----------------|
| **Terminal application** | `termui.TermApp` | Event loop, screen init, widget registry, redraw orchestration |
| **Root layout** | `termui` (planned `RootLayout`) | Fixed TabBar, flexible Workspace, fixed CmdLine |
| **Split tree** | `termui.Layout`, `WidgetTree`, `Node` | Recursive pane division inside Workspace |
| **Widget layer** | `termui.Widget` implementations | Per-pane UI and input handling |
| **Rendering** | `Canvas`, `Grid`, `Cell` | Local coordinates, border composition, terminal flush |
| **Domain events** | `core.Event` bus | Decouple widgets from app logic; all events → `HandleCoreEvents` |
| **Text model** | `core.Buffer`, `core.Viewport` | Scrollable line storage for console/source views |
| **Debugger backend** | `gdb.GDBClient`, `core.Debugger` | PTY I/O, MI2 parsing |
| **Legacy chat TUI** | `internal/ui/tui` | Separate Bubble Tea stack — not part of NewCGDB |

---

## Data flow

### Input → action → redraw

NewCGDB uses **two parallel event planes**:

| Plane | Type | Path |
|-------|------|------|
| **Terminal** | `tcell.Event` | `PollEvent` → `HandleUIEvent` + widget `HandleEvent` |
| **Domain** | `core.Event` | Any producer → `TermApp.events` channel → **`HandleCoreEvents`** |

Widgets handle terminal input locally (keys, cursor). When a widget needs the application to act — submit a `:` command, quit, forward to GDB — it **publishes** a `core.Event` onto the bus. The main loop drains the channel and forwards every domain event to a single application hook: `AppApi.HandleCoreEvents`.

```mermaid
sequenceDiagram
    participant Input as Keyboard / Mouse
    participant App as TermApp
    participant Widget as Widget · e.g. CmdWidget
    participant Bus as core.Event channel
    participant Core as HandleCoreEvents
    participant Render as Redraw

    Input ->> App: PollEvent · tcell.Event
    App ->> Widget: HandleEvent(ev)
    Widget ->> Bus: Events <- SubmitMsg / other core.Event
    App ->> Bus: drain channel
    Bus ->> Core: AppApi.HandleCoreEvents(ev)
    Core ->> Core: dispatch by CommandID / type
    App ->> Render: Draw → Grid → Screen
```

*Sources: [`diagrams/event_flow.mermaid`](diagrams/event_flow.mermaid) · [`diagrams/event_bus.mermaid`](diagrams/event_bus.mermaid)*

### Debugger output → UI

GDB output arrives asynchronously on a goroutine, is posted back into the tcell event loop via `EventInterrupt`, and is parsed into display buffers.

```mermaid
sequenceDiagram
    participant GDB as GDB process
    participant PTY as PTY reader goroutine
    participant Screen as tcell.Screen
    participant Widget as GDBWidget
    participant Buf as core.Buffer

    GDB-->>PTY: MI output chunk
    PTY->>Screen: PostEvent(EventInterrupt GdbOutputMsg)
    Screen->>Widget: HandleEvent
    Widget->>Widget: GdbInputState.PushLine
    Note over Widget: Timer fires → MiMsg batch
    Widget->>Buf: AppendBuffer
    Widget->>Widget: Draw on next frame
```

*Source: [`diagrams/debugger_integration.mermaid`](diagrams/debugger_integration.mermaid)*

### End-to-end data flow

```mermaid
flowchart TB
    subgraph Input["Input paths"]
        User["User keyboard / mouse"]
        Async["Async sources · GDB PTY, timers"]
    end

    subgraph TermApp["TermApp event loop"]
        Poll["PollEvent · tcell.Event"]
        Bus["events chan · core.Event"]
        UIHandler["HandleUIEvent"]
        Widgets["widget HandleEvent"]
        CoreHub["HandleCoreEvents"]
        Draw["Draw pipeline"]
        Screen["Terminal screen"]
    end

    subgraph App["Application layer"]
        Dispatch["Command / event dispatch"]
        Debugger["Debugger backend"]
        Model["Buffer / Viewport / state"]
    end

    User --> Poll
    Async --> Poll
    Poll --> UIHandler
    Poll --> Widgets
    Widgets -->|"publish domain events"| Bus
    Async -.->|"planned: publish"| Bus
    Bus --> CoreHub --> Dispatch
    Dispatch --> Debugger
    Debugger --> Model
    Dispatch --> Model
    Widgets --> Draw
    Draw --> Screen
```

*Source: [`diagrams/data_flow.mermaid`](diagrams/data_flow.mermaid)*

**Design decision:** domain events do **not** fan out to widgets directly. Every `core.Event` on the bus is handled in one place — `HandleCoreEvents` on the application object (`DebuggerApp` in `cmd/uitcell/main.go`). The app decides whether to exit, talk to GDB, change layout, or push state back into widgets on the next draw.

---

## Design principles

These principles are **non-negotiable** for NewCGDB. They explain many seemingly verbose abstractions (Canvas, WidgetTree, Grid).

| # | Principle | Rationale |
|---|-----------|-----------|
| 1 | Widgets should not know screen coordinates | Enables layout changes without touching widget code |
| 2 | Widgets draw only inside their assigned `Rect` | Prevents bleed-over; simplifies testing |
| 3 | `Canvas` provides local drawing coordinates | Single translation point from local to screen space |
| 4 | Layout engine owns positioning | Centralizes split ratios, resize, and border gutters |
| 5 | Rendering backend should be replaceable | Grid → tcell today; could swap to alternate terminal libs |
| 6 | Business logic separated from rendering | `core` has no tcell imports |
| 7 | TabBar, CmdLine, Workspace are top-level | Split tree stays scoped to Workspace only |
| 8 | Only Workspace contains the split tree | TabBar/CmdLine never participate in recursive splits |
| 9 | Debugger backends must not import UI | Keeps GDB/OpenOCD/JTAG testable without a terminal |

---

## Layer responsibilities

### Presentation (`internal/termui`)

- Owns `tcell.Screen` lifecycle.
- Registers top-level widgets (today: flat list; target: structured Root).
- Runs the poll/draw loop.
- Must **not** parse GDB MI records directly — delegates to widgets that use `internal/gdb`.

### Application (`cmd/uitcell`, `internal/app`)

- **NewCGDB:** `DebuggerApp` embeds `termui.TermApp`, implements `AppApi`, and owns **`HandleCoreEvents`** — the single dispatch hub for all `core.Event` traffic.
- **Legacy chat TUI:** `internal/app` connects Bubble Tea models to core via `HandleEvent`.
- Defines interaction modes (`NormalMode`, `InsertMode`, `CommandMode`) — constants in `app/modes.go`; wiring into `TermApp` is in progress.

### Domain (`internal/core`)

- **`Event` bus types** — `Event`, `CommandEvent`, `SubmitMsg`, `GdbOutputMsg`, etc.
- **`CommandID`** — infra constant `CmdUnknown` only; app-specific command IDs live in the application package.
- `Buffer`, `Viewport` for text panes.
- `History`, `AutoCompleter` for command-line UX.
- `Debugger` interface — minimal send API.

### Infrastructure (`internal/gdb`)

- Spawns GDB with `--interpreter=mi2`.
- Reads PTY output, publishes `core.GdbOutputMsg`.
- Parses MI lines into `MiMsg`, `GdbInputState`.

**Dependency rule:** `termui` → `core` ← `gdb`. Never `gdb` → `termui`.

---

## Core events layer

The **event bus** decouples UI widgets from application logic. Any subsystem may publish a `core.Event`; the main loop delivers every event to **`HandleCoreEvents`** on the application object. Widgets stay thin — they parse local input and emit domain events; the app owns side effects.

```mermaid
flowchart TB
    subgraph Producers["Event producers (any subsystem)"]
        CmdW["CmdWidget"]
        GDB["GDB backend / goroutines"]
        Widgets["Other widgets"]
        Future["Plugins / timers · planned"]
    end

    subgraph Bus["core event bus"]
        Chan["TermApp.events chan core.Event"]
    end

    subgraph Loop["TermApp.Run main loop"]
        Select["select: channel vs PollEvent"]
        CoreDispatch["AppApi.HandleCoreEvents(ev)"]
    end

    subgraph App["Application · DebuggerApp"]
        Handler["HandleCoreEvents — single dispatch hub"]
        CmdSwitch["switch CommandID / event type"]
    end

    CmdW -->|"Events <- SubmitMsg"| Chan
    GDB -.->|"planned"| Chan
    Widgets -.-> Chan
    Chan --> Select --> CoreDispatch --> Handler --> CmdSwitch
```

*Source: [`diagrams/event_bus.mermaid`](diagrams/event_bus.mermaid)*

### Interfaces and types

```mermaid
classDiagram
    direction TB

    class Event {
        <<interface>>
        +Type() string
    }

    class CommandEvent {
        <<interface>>
        +CommandID() CommandID
    }

    class SubmitMsg {
        +Text string
        +CmdID CommandID
        +Args string
    }

    class GdbOutputMsg {
        +Data string
        +Err error
    }

    class Quit {
        +Text string
    }

    Event <|.. SubmitMsg
    Event <|.. GdbOutputMsg
    Event <|.. Quit
    CommandEvent <|.. SubmitMsg
```

| Type | Purpose |
|------|---------|
| `Event` | Base domain event — identified by `Type() string` |
| `CommandEvent` | Events carrying a resolved `CommandID` (e.g. after `:` command entry) |
| `SubmitMsg` | CmdLine submitted — `Text`, `CmdID`, `Args` |
| `GdbOutputMsg` | Raw GDB output for UI consumption |
| `Quit` | Application exit request |
| `SubmitMessage`, `RunCommand` | Legacy chat TUI types — retained for `internal/ui/tui` |

### Command IDs

| Layer | Owns |
|-------|------|
| **`core`** | `CommandID` type, **`CmdUnknown`** (value `0`) |
| **Application** (`cmd/uitcell`) | Private constants: `cmdBreak`, `cmdQuit`, … starting at `iota + 1` |

`CmdWidget` never references app command names. It resolves user input through `core.AutoCompleter` and emits `SubmitMsg{CmdID: …}`. Unknown commands emit `core.CmdUnknown`.

### Wiring

```go
// termui/term_app.go
type AppApi interface {
    HandleUIEvent(ev tcell.Event)      // terminal-level hooks · resize layout
    HandleCoreEvents(ev core.Event)    // all domain events land here
}

// cmd/uitcell/main.go
cmd.Events = a.Events()   // CmdWidget publishes to TermApp channel

func (app *DebuggerApp) HandleCoreEvents(ev core.Event) {
    switch msg.CommandID() {
    case core.CmdUnknown: /* feedback */
    case cmdQuit:         app.Exit()
    }
}
```

Implementation: `internal/core/events.go`, `internal/core/command.go`, `internal/termui/term_app.go`, `internal/termui/cmd_widget.go`.

---

## Current vs target architecture

The **target** architecture is documented across this tree. The **current** codebase is mid-migration.

| Component | Target | Current state |
|-----------|--------|---------------|
| Root layout | TabBar + Workspace + CmdLine | Widgets registered flat on `TermApp` |
| TabBar | Multi-tab with header render | `TabWidget` — single tab, no header |
| Workspace | Split tree only | `Layout` / `WidgetTree` implemented |
| CmdLine | Global `:` command input | `CmdWidget` — draw, history, emits `SubmitMsg` on event bus |
| Event bus | `core.Event` → `HandleCoreEvents` | Channel on `TermApp`; `CmdWidget` wired; GDB publish planned |
| Rendering | Diff-based grid flush | Full grid draw every frame |
| Focus | Mode-aware routing | `WidgetTree.focus` — partial |
| Debugger | Abstract backend + GDB | GDB PTY prototype in `GDBWidget` |

Entry point: `cmd/uitcell/main.go`.

Detailed tracker: [ROADMAP.md](ROADMAP.md).

---

## Related documentation

| Topic | Document |
|-------|----------|
| Widgets, canvas, grid | [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) |
| Splits, tabs, command line | [WINDOW_MANAGEMENT.md](WINDOW_MANAGEMENT.md) |
| Cells, borders, Unicode | [RENDERING.md](RENDERING.md) |
| Keyboard, modes | [INPUT.md](INPUT.md) |
| GDB MI2 | [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) |
| Package map | [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md) |

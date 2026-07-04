# Window Management

cgdb-go organizes debugger panes through a **Workspace** containing a recursive **split tree**, managed at the top level by **tabs** and a global **command line**. A future **status bar** will sit below the workspace or above the command line.

**Companion docs:** [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [INPUT.md](INPUT.md) · [ARCHITECTURE.md](ARCHITECTURE.md)

---

## Table of contents

- [Top-level layout](#top-level-layout)
- [Workspace concept](#workspace-concept)
- [Split tree architecture](#split-tree-architecture)
- [Horizontal and vertical splits](#horizontal-and-vertical-splits)
- [Splitting at runtime](#splitting-at-runtime)
- [Tab management](#tab-management)
- [Command line](#command-line)
- [Buffer command](#buffer-command)
- [Future status bar](#future-status-bar)
- [Planned window operations](#planned-window-operations)

---

## Top-level layout

The root UI is **not** a split tree. It is a fixed vertical stack:

```text
Root
├── TabBar      (fixed height)
├── Workspace   (remaining area — contains split tree)
└── CmdLine     (fixed height)
```

```text
+--------------------------------------------------+
| Tab1 | Tab2 | Tab3                               |
+--------------------------------------------------+
|                                                  |
|                 Workspace                        |
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

**Design decision:** keeping TabBar and CmdLine **outside** the split tree means:

- Tabs always remain visible regardless of pane layout.
- The command line is a stable anchor (like Vim's `:` line).
- Workspace resize math is isolated — only the middle band changes height on terminal resize.

**Current gap:** `TermApp` registers widgets in a flat slice rather than a structured `RootLayout`. **`DebuggerApp.HandleResize`** assigns rects today:

```go
w[0].SetRect(c.ChildRect(0, 0, c.W(), c.H()))       // TabWidget (workspace)
w[1].SetRect(c.ChildRect(0, c.H()-1, c.W(), 1))     // CmdWidget (bottom row)
```

Called on startup (`NewDebuggerApp`) and on every `EventResize`. Migration path: introduce `RootWidget` that owns the three bands internally.

---

## Workspace concept

The **Workspace** is the rectangular region between TabBar and CmdLine. It is the **only** place where recursive splits exist.

Workspace panes are **widgets** — views bound to application **models** that were created at startup. Typical GDB models and their views:

| Model | Widget (view) | Purpose |
|-------|---------------|---------|
| `CodeModel` | Source view | Current file, breakpoint markers, PC highlight |
| `BreakpointModel` | Breakpoints pane | Breakpoint list, enable/disable |
| `ConsoleModel` | Console pane | GDB / target output |
| `RegisterModel` | Registers pane | Register dump |
| `MemoryModel` | Memory pane | Hex memory view |
| `ThreadModel` | Threads pane | Thread list |
| `LoggerModel` | Logger pane | Application log |

```text
Workspace
└── SplitTree
    ├── CodeWidget        → CodeModel
    ├── BreakpointWidget  → BreakpointModel
    ├── ConsoleWidget     → ConsoleModel
    └── …
```

Each pane is a **leaf widget** in the split tree. The Workspace does not draw content itself — it delegates geometry to `WidgetTree.BuildLayout`. Models exist whether or not a widget is currently displaying them.

---

## Split tree architecture

The split tree is a **full binary tree** where internal nodes are splits and leaves are widgets.

```mermaid
graph TB
    WS["Workspace"]
    VS["VerticalSplit"]
    SRC["Source View"]
    HS["HorizontalSplit"]
    BP["Breakpoints"]
    CON["Console"]

    WS --> VS
    VS --> SRC
    VS --> HS
    HS --> BP
    HS --> CON
```

*Source: [`diagrams/split_tree.mermaid`](diagrams/split_tree.mermaid)*

Example ASCII layout matching the diagram above:

```text
Workspace
└── VerticalSplit
    ├── Source
    └── HorizontalSplit
        ├── Breakpoints
        └── Console
```

### Node types

```text
Node
├── Leaf (Widget)
└── Split
    ├── Horizontal  (top / bottom)
    └── Vertical    (left / right)
```

**Design decision:** binary splits (not n-way splits) simplify ratio math and border drawing. An n-way toolbar layout can be built by composing binary nodes — the same approach used by Emacs window management and many IDE dock systems.

---

## Horizontal and vertical splits

| `SplitDir` | Orientation | First child position | Separator |
|------------|-------------|----------------------|-----------|
| `Vertical` | Left \| Right | Left | Vertical line (`│`) |
| `Horizontal` | Top / Bottom | Top | Horizontal line (`─`) |

Layout algorithm (`widget_tree.go`):

1. Compute first-child size using **`Units()`** leaf weighting along the split axis.
2. Reserve 1 cell for the separator.
3. Assign remaining space to second child.
4. Recurse into both children with child `Canvas` values (`WithRect`).

**Ratio default:** `0.5` when a split is created via `WidgetTree.Split`. `BuildLayout` currently uses unit counts, not `Ratio` directly.

**Border drawing:** separators are written into the shared `Grid` during `BuildLayout`, not by individual widgets. This ensures corners align when splits nest.

---

## Splitting at runtime

```go
layout := NewLayout(initialWidget)
layout.NewSplit(Vertical, newWidget)  // splits focused pane
```

What happens:

1. The focused leaf becomes a split node.
2. `First` retains the original widget; `Second` gets the new widget.
3. Focus moves to `First` (original pane).
4. `Ratio` is set to `0.5`.

**Design rationale:** splitting at focus matches cgdb/emacs user expectations. Alternative designs (split always right, pick target pane first) may be added as commands (`:vsplit`, `:hsplit`) later.

**Current gap in `TabWidget`:** `NewTabTwoHozSplitWins` accepts two widgets but only passes the first to `NewLayout`; the second widget is not yet installed in a split. Use `:vs` / `:split` at runtime or call `layout.NewSplit` directly for a working two-pane layout.

---

## Tab management

Each tab owns an independent **Workspace layout** (a `Layout` / `WidgetTree` instance).

```mermaid
flowchart LR
    TabBar["TabBar"]
    T1["Tab 1 · Layout"]
    T2["Tab 2 · Layout"]
    T3["Tab 3 · Layout"]

    TabBar --> T1
    TabBar --> T2
    TabBar --> T3
```

Current `TabWidget` implementation:

```go
type Tab struct {
    Title  string
    Layout *Layout
}

type TabWidget struct {
    tabs   []Tab
    active int
}
```

| Feature | Status |
|---------|--------|
| Single tab container | Implemented |
| Forward events/draw to active tab | Implemented |
| Tab header rendering | Not implemented |
| Tab switching | Not implemented |
| Tab close / new tab | Not implemented |
| Persist layout per tab | Not implemented |

**Design decision:** tabs are **workspace presets**, not separate debugger sessions (initially). A tab might represent "source + console" vs "registers + memory". Multi-session tabs may come later with backend association per tab.

`TabWidget.Draw` currently shaves 2 rows from height (`r.H()-2`) — a temporary adjustment pending proper Root layout integration.

---

## Command line

The **CmdLine** is a top-level band for **Vim-style `:` commands**, distinct from the GDB `(gdb)` prompt inside a console pane.

```text
+--------------------------------------------------+
| : break main                                     |
+--------------------------------------------------+
```

`CmdWidget` (`cmd_widget.go`) provides:

- Vim-style `:` activation and drawing on the bottom line (row `H-1` of the terminal).
- Command history (`termui.History`) — Up/Down navigation.
- Tab completion (`termui.AutoCompleter`) — command name only.
- **`SubmitMsg` on the event bus** — resolved `CommandID` + args; app dispatches in `HandleCoreEvents`.

Command mode is entered by **`DebuggerApp`** (`:` → `ModeCommand`, `CmdWidget.Activate()`), not by `CmdWidget` alone. `Esc` returns to normal mode at the app layer.

**Current state:** `:quit` closes focused pane or exits via `HandleCoreEvents`. Split commands (`:vs`, `:split`) partially wired. Unknown commands emit `termui.CmdUnknown`.

**Design decision:** separate CmdLine from GDB console because:

- GDB console speaks MI/cli dialect; CmdLine speaks **UI commands** (`:split`, `:focus`, `:tabnew`, `:quit`).
- Users can run UI operations without sending spurious input to GDB.
- Completion vocabularies differ (UI vs debugger).

Planned flow details: see [INPUT.md](INPUT.md#vim-like-command-system) and [ARCHITECTURE.md](ARCHITECTURE.md#buffer-concept).

---

## Buffer command

The `:buffer` command selects which **application model** to display — it does not open a text file.

```text
:buffer code
:buffer breakpoints
:buffer threads
:buffer registers
:buffer memory
:buffer console
:buffer logger
```

Each name corresponds to a model declared at application startup. Running `:buffer` creates (or activates) a widget bound to that model in the focused window, or replaces the focused pane's view.

**Related window commands** (same model-binding semantics):

| Command | Action |
|---------|--------|
| `:buffer <name>` | Display model in focused pane |
| `:split` / `:vsplit` | Split focused pane; new pane also bound via subsequent `:buffer` or default |
| `:tab` | Open model in a new tab workspace |

The architecture does **not** use `:attach <name>`. All models exist from startup; the user only chooses which to display. See [ARCHITECTURE.md](ARCHITECTURE.md#why-not-attach).

**Current state:** `:buffer` dispatch is planned; the prototype creates widgets directly in layout at init. Model types per domain are not yet fully separated from widget state.

---

## Future status bar

A **status bar** is planned between Workspace and CmdLine (or integrated into CmdLine's opposite edge):

```text
+--------------------------------------------------+
| Workspace …                                      |
+--------------------------------------------------+
| Normal | main.c:42 | stopped | thread 1          |  ← StatusBar
+--------------------------------------------------+
| :                                                |
+--------------------------------------------------+
```

Planned contents:

| Segment | Source |
|---------|--------|
| Mode | `Normal` / `Focus` / `Command` |
| Location | Current file:line from debugger |
| Target state | `running` / `stopped` from MI `*stopped` |
| Thread | Active thread ID |
| Backend | `GDB` / `OpenOCD` indicator |

**Design decision:** status bar is **read-only** and outside the split tree — it never steals focus. Updates arrive via `core.Event` from debugger backends, not from widget polling.

---

## Planned window operations

| Command | Action |
|---------|--------|
| `:split` / `:vsplit` | Split focused pane horizontally / vertically |
| `:close` | Close focused pane (collapse split) |
| `:focus left/right/up/down` | Move focus |
| `:tabnew` / `:tabclose` / `:tabn` | Tab management |
| `:only` | Collapse to single pane |
| `:resize +N/-N` | Adjust split ratio |

These will be implemented in the command router ([INPUT.md](INPUT.md)) and call into `Layout` / `WidgetTree` APIs.

---

## Related documentation

- [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) — layout engine internals
- [INPUT.md](INPUT.md) — focus and command mode
- [RENDERING.md](RENDERING.md) — split border drawing

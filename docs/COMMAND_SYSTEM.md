# Command System

gdbforge routes user actions through a **hierarchical command tree**. Colon commands (`:window left`), tab completion, and key chords (`Ctrl+W h`) all resolve against the same `CommandNode` types, but through different entry points.

**Companion docs:** [INPUT.md](INPUT.md) · [ARCHITECTURE.md](ARCHITECTURE.md) · [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) · [EXEC_SHELL.md](EXEC_SHELL.md)

---

## Table of contents

- [Ownership model](#ownership-model)
- [CommandNode — the tree](#commandnode--the-tree)
- [CommandRegistry — owns the tree](#commandregistry--owns-the-tree)
- [CommandParser — navigates the tree](#commandparser--navigates-the-tree)
- [DSL — building the tree](#dsl--building-the-tree)
- [Rest-args leaves](#rest-args-leaves)
- [CmdWidget integration](#cmdwidget-integration)
- [Tab completion presenter](#tab-completion-presenter)
- [Key bindings](#key-bindings)
- [Adding commands](#adding-commands)

---

## Ownership model

Three types work together. Each has a distinct responsibility — do not conflate them.

```mermaid
flowchart TB
    subgraph static ["Built once at startup"]
        Registry["CommandRegistry"]
        Root["Root CommandNode /"]
        Tree["CommandNode tree"]
        Registry --> Root
        Root --> Tree
    end

    subgraph dynamic ["Per keystroke / per command line"]
        Parser["CommandParser"]
        Current["current *CommandNode"]
        Token["token string"]
        Path["path []*CommandNode"]
        Parser --> Current
        Parser --> Token
        Parser --> Path
    end

    subgraph ui ["UI layer"]
        CmdW["CmdWidget"]
        Bus["platform.EventBus"]
        Bar["CompletionBarWidget"]
        CmdW --> Parser
        CmdW -->|"Publish CompletionMsg"| Bus
        Bus -->|"Subscribe"| Bar
    end

    Parser -->|"references"| Registry
    DSL["ExapData() · Group/Cmd"] -->|"populates"| Tree
```

| Type | Role | Owns / holds |
|------|------|----------------|
| **`CommandNode`** | One node in the hierarchy | `Name`, `Children` trie, optional `Action`, optional `RestArgs` |
| **`CommandRegistry`** | Application command catalog | `Root` node (the tree) + `Keys` trie for key chords |
| **`CommandParser`** | Runtime cursor over the tree | `current`, `token`, `path` — **does not own the tree** |

**Mental model:**

- **`CommandNode`** *is* the tree (data).
- **`CommandRegistry`** *owns* the tree (storage).
- **`CommandParser`** *navigates* the tree (state while the user types).

The tree is built once at startup (via the DSL or `Insert`). The parser resets and replays input on each Tab or Enter; it never mutates tree structure.

---

## CommandNode — the tree

Each node is either a **container** (has children, no action) or a **leaf** (has an `Action` callback).

```text
/  (root)
├── window
│   ├── left      → Action: OnFocusLeft
│   ├── right     → Action: OnFocusRight
│   ├── up        → Action: OnFocusUp
│   └── down      → Action: OnFocusDown
├── gdb
│   ├── break
│   │   ├── file  → Action: BreakFile
│   │   └── delete → Action: DeleteBreakpoint
│   └── info
│       ├── registers → Action: ShowRegisters
│       └── threads   → Action: ShowThreads
├── b <name>      → Action: OnBuffer (about/logger/gdb/breakpoint/threads/callstack/output/exec or open file)
├── edit [name]   → Action: OnEdit (picker, or open file; unique prefix :e)
├── layout <name> → Action: OnLayout (default | panels | classic)
├── set           → equalalways | noequalalways | clearoutput | noclearoutput | continueafterclear | nocontinueafterclear | esctocode | noesctocode | breakmain | nobreakmain | gdblistenprint | nogdblistenprint | gdbtargetprint | nogdbtargetprint | inferior-tty <path> | markcolor <name> | markdimcolor <name> | breakcolor <name> | breakdisabledcolor <name>
├── vs            → Action: SplitVertical
├── split         → Action: SplitHorizontal
├── clear         → Action: ClearFocus
└── quit          → Action: Quit
```

```go
type CommandNode struct {
    Parent   *CommandNode
    Children *collections.Trie[*CommandNode]
    Name     string
    Action   func(args ...any)
}
```

Children are stored in a **trie** (key-sequence trie reused for name lookup) so prefix completion is efficient: `Complete("win")` under root returns `[window]`.

| Node kind | `Action` | `Children` | Example |
|-----------|----------|------------|---------|
| Container | `nil` | non-empty | `window`, `gdb`, `break` |
| Leaf | set | may be empty | `left`, `delete`, `registers` |

---

## CommandRegistry — owns the tree

```go
type CommandRegistry struct {
    Root *CommandNode
    Keys *collections.Trie[*CommandNode]  // key chord → command node
}
```

- **`Root`** — entry point for colon-command parsing (`/`).
- **`Keys`** — separate trie for keyboard bindings (`<C-w>h` → move-right). Used by `KeyBindingRegistry`, not by `CommandParser`.

The application holds one `CommandRegistry` on `DebuggerApp` and passes it to `NewCmdWidget` and `InitKeyBindings`.

---

## CommandParser — navigates the tree

The parser holds a **pointer into** the registry tree plus parse state:

```go
type CommandParser struct {
    registry *CommandRegistry
    current  *CommandNode   // position in tree (parent context for current token)
    token    string         // partial word being typed
    path     []*CommandNode // accepted nodes so far
}
```

### Parser state by example

| User input (after `:`) | `current` | `token` | `path` |
|------------------------|-----------|---------|--------|
| *(empty)* | `/` (root) | `""` | `[]` |
| `win` | `/` | `"win"` | `[]` |
| `window ` | `window` | `""` | `[window]` |
| `window l` | `window` | `"l"` | `[window]` |
| `window left` (after Enter) | `left` | `""` | `[window, left]` |

### When `current` changes

`current` moves only inside **`Accept()`**:

```go
p.current = list[0]
p.path = append(p.path, p.current)
p.token = ""
```

| Trigger | Calls `Accept()`? |
|---------|-------------------|
| **Tab** | Indirectly — `Sync()` replays finished tokens (before spaces); `Suggestions()` does not move `current` |
| **Enter** | Yes — `Parse()` accepts every token |
| **Typing** | No — text lives in `CmdWidget`; parser catches up on next Tab via `Sync()` |

`current` is the **parent context** for the token under the cursor, not the highlighted suggestion. While typing `:window l`, `current` is `window` and suggestions are `[left, right, up, down]`.

### Key methods

| Method | Purpose |
|--------|---------|
| `Sync(line, cursor)` | Replay input up to cursor; rebuild `current`, `token`, `path` |
| `Suggestions()` | `current.Complete(token)` — tab completion candidates |
| `Accept()` | Move `current` to the single matching child |
| `Parse(line)` | Reset + accept all tokens (on Enter) |
| `CanExecute()` | `current.Action != nil` |
| `Execute()` | Call `current.Action` |

---

## DSL — building the tree

The DSL in `internal/commands/dsl.go` builds the tree declaratively instead of imperative `InsertName` calls.

| Function | Creates |
|----------|---------|
| `Cmd(name, action)` | Leaf node with `Action` (not yet in tree) |
| `CmdRest(name, action)` | Leaf with `RestArgs` — remainder of line → action args |
| `Group(name, children...)` | Container node with children attached |
| `(n *CommandNode).Group(name, children...)` | Inserts a group into `n`, returns `n` for chaining |
| `(n *CommandNode).Leaf` / `LeafRest` | Insert leaf (or rest-args leaf) into `n`, return `n` |

### Example (`DebuggerApp.ExapData` in `cmd/gdbforge/command_tree.go`)

```go
func (a *DebuggerApp) ExapData() {
    a.commandReg.Root.
        Group("window",
            commands.Cmd("left", a.OnFocusLeft),
            commands.Cmd("right", a.OnFocusRight),
            commands.Cmd("up", a.OnFocusUp),
            commands.Cmd("down", a.OnFocusDown),
        ).
        Group("gdb",
            commands.Group("break",
                commands.Cmd("file", a.BreakFile),
                commands.Cmd("delete", a.DeleteBreakpoint),
            ),
            commands.Group("info",
                commands.Cmd("registers", a.ShowRegisters),
                commands.Cmd("threads", a.ShowThreads),
            ),
        ).
        // ...
}
```

Equivalent imperative form (older style, same result):

```go
window := root.InsertName("window")
window.InsertName("left").Action = a.OnFocusLeft
// ...
```

The DSL reads as a nested outline: groups and commands mirror the logical hierarchy (`window` → `left`) instead of spelling out each parent/child link.

### DSL rules

1. **`Cmd`** — always a leaf; must be placed inside a `Group` (or root `.Group()`).
2. **`Group`** — container; may nest other `Group` calls for deeper paths (e.g. `window` → `split` → `horizontal`).
3. **Chaining** — `.Group()` on `Root` returns `Root`, so sibling groups chain at the same level.
4. **No tree mutation at runtime** — the DSL runs once during `InitB`; the parser only reads the result.

---

## Rest-args leaves

Some commands need a free-form tail (shell argv, file paths). Those use **`RestArgs`**:

| DSL | Effect |
|-----|--------|
| `Cmd(name, action)` / `Leaf` | Fixed leaf; further tokens must be children |
| `CmdRest(name, action)` / `LeafRest` | Sets `CommandNode.RestArgs = true` |

After `Accept()` lands on a rest-args node, `Parse` **stops walking** and stores remaining tokens in `parser.args`. `Execute()` passes them to `Action`.

```text
:!ssh root@host
  │ └──────────┘
  │    args (not Accept()'d)
  └─ current stays on "!" → OnRun
```

Vim-style bang is registered as `LeafRest("!", a.OnRun)` and has an extra Parse/Sync path so `:!ls` (no space) works. Full product docs: [EXEC_SHELL.md](EXEC_SHELL.md).

In-app AI uses the same rest-args pattern: `LeafRest("AI", a.OnAI)` / `LeafRest("ai", a.OnAI)` — see [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#gdbmcpservice-and-in-app-ai).

Vim-like buffers use the same pattern:

| Command | Handler | Behavior |
|---------|---------|----------|
| `:help` | `Leaf("help", OnHelp)` | Open the Viewport user manual in the focused pane (same widget as `:b help`). |
| `:b name` | `LeafRestComplete("b", OnBuffer, bufferCompletions)` | Switch to builtin (`help`, `about`, `logger`, `gdb`, `breakpoint`, `threads`, `callstack`, `output`, `exec`) or an **already open** file CodeWidget. **Tab** lists builtins + open file buffers (dynamic). |
| `:edit` / `:edit name` | `LeafRestComplete("edit", OnEdit, editCompletions)` | No args: project source picker (FileListWidget). With a name: open that source CodeWidget. **`:e`** is the unique prefix of `:edit` (same command). **Tab** lists `SourceFiles` full paths. Does **not** pollute `:b`. |
| `:layout name` | `LeafRestComplete("layout", OnLayout, layoutCompletions)` | Apply a registered workspace: **`wide`** (startup: Code\|IO over GDB\|(Threads\|Callstack / BP)), **`panels`**, **`default`** (six-pane), **`classic`** (Code over GDB). Builders in `internal/gdbforge/layout`. Bare `:layout` re-applies **wide**. |
| `:set clearoutput` / `:set noclearoutput` | `Cmd` under `set` | Clear IO pane on GDB session Start (default **on**). Does **not** clear on step/`n`. |
| `:set continueafterclear` / `:set nocontinueafterclear` | `Cmd` under `set` | After removing a breakpoint while the inferior was running, resume with `continue` (default **off** — stay stopped). Inserting a breakpoint still auto-continues; `frame`/`thread` never auto-continue. |
| `:set esctocode` / `:set noesctocode` | `Cmd` under `set` | Esc restores the last non-Code/non-GDB pane when one was focused, otherwise focuses the CodeWidget leaf (default **on**). With `noesctocode`, Esc only leaves insert → normal and keeps the current pane focused. |
| `:set breakmain` / `:set nobreakmain` | `Cmd` under `set` | Insert `break main` when the GDB session starts (default **on**). Skipped when restoring `./.gdbforge/breakpoints.yaml` or when GDB args include `-x`/`-ex` (`HasInitScript`). `:set breakmain` also inserts immediately if a session is already live. |
| `:set gdbtargetprint` / `:set nogdbtargetprint` | `Cmd` under `set` | Also show program stdout in the GDB console (legacy; default **off** except Delve). |
| `:set inferior-tty` / `:set inferior-tty internal` | `CmdRestComplete` under `set` | No arg: open external terminal and route program stdio there. `internal` restores `:b io`. Optional `/dev/pts/N`. Tab: `internal`. **GDB:** live `-inferior-tty-set`. **Delve:** restarts with `--tty` (prefer `:lua dlv_port` for Go TUIs). |
| `:set markcolor <name>` | `CmdRest` under `set` | Focused selected-row color for list panes / file picker (default **blue**). |
| `:set markdimcolor <name>` | `CmdRest` under `set` | Unfocused selected-row color for list panes (default **gray**). |
| `:set breakcolor <name>` | `CmdRest` under `set` | Enabled breakpoint background in CodeWidget gutter and BreakpointWidget (default **red**). |
| `:set breakdisabledcolor <name>` | `CmdRest` under `set` | Disabled breakpoint background (default **yellow**). |
| `:set pccolor <name>` | `CmdRest` under `set` | Code ━━▶ (StopLocation) row background (default **darkslategray**). |
| `:set stackbreakcolor <name>` | `CmdRest` under `set` | Green mark for BP / Call Stack #0 / current Thread at stop PC (default **green**). |
| `:set codeselcolor <name>` | `CmdRest` under `set` | Code browse-cursor background (default **darkblue**). |
| `:set mutedcolor <name>` | `CmdRest` under `set` | Empty-list / dim text (default **gray**). |

Startup **`panels`** layout: Code over GDB (left, **2/3**); right IO (top half) and (Threads\|Callstack) over Breakpoints (bottom half). **`default`** is the six-pane workspace; **`classic`** is full-width Code/GDB. Program stdin/stdout: `:b io` (alias `:b output`). Breakpoint list: `:b breakpoint` — see [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#breakpoints-and-source-sync). Persist: [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#breakpoint-persistence).

---

## CmdWidget integration

`CmdWidget` (`internal/termui/cmd_widget.go`) is a **muxed** cmdline: **`CmdKindCommand`** (`:`) and **`CmdKindSearch`** (`/`). It holds a `CommandParser` for Tab sync / completions on the command kind only (search has its own history and **no** Tab). **Execute** for `:` is owned by the app (`SetOnExecute` → `ExecuteParsed()`). Search Enter / live edits call `SetOnSearchSubmit` / `SetOnChange` → focused pane `SearchHost` ([INPUT.md](INPUT.md)).

```mermaid
sequenceDiagram
    participant User
    participant CmdW as CmdWidget
    participant Parser as CommandParser
    participant App as DebuggerApp
    participant Bus as platform.EventBus
    participant Bar as CompletionBarWidget
    participant Tree as CommandNode tree

    User->>CmdW: Tab
    CmdW->>Parser: Sync(text, cursor)
    Parser->>Tree: replay tokens, Accept on spaces
    CmdW->>Parser: SuggestionNames()
    Parser-->>CmdW: names
    CmdW->>Bus: Publish(CompletionMsg)
    Bus->>Bar: onCompletion / show wildmenu

    User->>CmdW: Enter
    CmdW->>Parser: Parse(line)
    Parser->>Tree: Accept each token
    CmdW->>App: onExecute()
    App->>CmdW: ExecuteParsed()
    CmdW->>Parser: Execute()
    Parser->>Tree: current.Action()
```

Wiring in `cmd/gdbforge/setup.go`:

```go
a.cmdWidget = termui.NewCmdWidget(a.commandReg)
a.cmdWidget.Ctx = a.ctx   // provides EventBus for CompletionMsg
a.cmdWidget.SetOnExecute(func() {
    _ = a.cmdWidget.ExecuteParsed()
})
a.completionBar = termui.NewCompletionBarWidget(a.ctx)
```

On **Enter**, the widget calls `Parse`; if `CanExecute()`, it invokes **`onExecute`** (app controller). Leaf actions run on the `CommandNode` — no `CommandID` / `SubmitMsg` indirection for tree commands.

---

## Tab completion via EventBus

Tab completion is announced as a **`termui.CompletionMsg`** on **`platform.EventBus`**:

```go
type CompletionMsg struct {
    Input string
    Token string
    Names []string
}
```

| Role | Where | Behavior |
|------|-------|----------|
| Publisher | `CmdWidget` (Tab) | `platform.Publish(ctx.Bus, CompletionMsg{…})` |
| Subscriber | `CompletionBarWidget` | Wildmenu row above `:` (white-on-black); multi-match → `ModeCompletion` |
| Keys | `ModeCompletion` | Left/Right/Up/Down cycle; Esc → `ModeCommand`; Enter applies token |

Single unique match still auto-inserts in `ModeCommand` (no mode switch). The bar is TermApp chrome (draw after `TabWidget`), not a `WidgetTree` leaf.

**Architecture note:** wildmenu is not a popup layer. It is the same chrome pattern as `CmdWidget` — `AddWidget` + `HandleResize` rect + mode-routed keys + draw-only-when-active. Future one-line overlays should follow that pattern; see [WINDOW_MANAGEMENT.md](WINDOW_MANAGEMENT.md#extending-chrome-no-popup-layer).

Producers depend only on the bus + message type. Consumers register independently (avoids constructor injection and cyclic wiring).

The same EventBus pattern drives breakpoint refresh: `BreakpointsChangedMsg` (see [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#breakpoints-and-source-sync)).

---

## Key bindings

Key chords use the **same `CommandNode` type** but a **different registry**:

| Entry point | Registry | Lookup |
|-------------|----------|--------|
| Colon commands | `CommandRegistry.Root` | `CommandParser` walks name tokens |
| Key chords | `KeyBindingRegistry` trie | `SearchPartial(key)` per keypress |

```go
// cmd/gdbforge/keybindings.go
func (a *DebuggerApp) InitKeyBindings() {
    a.keyBindings = commands.NewKeyBindingRegistry()
    a.keyBindings.Bind(
        commands.NewCommand("move-left", func(args ...any) { a.OnFocusLeft() }),
        "<C-w>l", "<C-w><Left>",
    )
    a.keyBindings.Bind(
        commands.NewCommand("jump-back", a.JumpBack),
        "<C-o>",
    )
}
```

A key binding can invoke the same handler as a colon command (`OnFocusLeft`) without sharing the parser — both are wired at init time in the application. `<C-o>` restores the previous pane widget after `:b` / `:e` / `:!` swaps — see [EXEC_SHELL.md](EXEC_SHELL.md).

---

## Adding commands

### Colon command (tree)

1. Add a `Cmd` or nested `Group` in `ExapData()` (`cmd/gdbforge/command_tree.go`).
2. Implement the handler on `DebuggerApp` in `cmd/gdbforge/actions.go`.
3. No `CommandID` or `HandleCoreEvents` wiring needed for tree leaves — `Execute()` calls `Action` directly.

### Key chord

1. Add `a.keyBindings.Bind(…)` in `cmd/gdbforge/keybindings.go`.

### Tab completion feedback

1. `CompletionBarWidget` subscribes to `termui.CompletionMsg` (wildmenu above the cmdline).

---

## Package layout

| Path | Responsibility |
|------|----------------|
| `internal/commands/command_node.go` | `CommandNode`, `CommandRegistry` |
| `internal/commands/command_parser.go` | `CommandParser` — navigation, completion, execution |
| `internal/commands/dsl.go` | `Cmd`, `CmdRest`, `Group`, `Leaf`, `LeafRest` builders |
| `internal/commands/key_binding_gegistry.go` | `KeyBindingRegistry` |
| `internal/termui/cmd_widget.go` | `:` input, parser sync, tab; `SetOnExecute` → app |
| `internal/termui/completion_bar.go` | Wildmenu chrome row; `ModeCompletion` nav |
| `internal/termui/event.go` | `CompletionMsg` and other UI-generic events |
| `cmd/gdbforge/events.go` | Debugger domain events (`BreakpointsChangedMsg`) |
| `internal/platform/event_bus.go` | Typed `Subscribe` / `Publish` |
| `internal/termui/logger_widget.go` | Log sink pane |
| `cmd/gdbforge/command_tree.go` | `ExapData` DSL |
| `cmd/gdbforge/keybindings.go` | `InitKeyBindings` |
| `cmd/gdbforge/actions.go` | Command action methods |
| `cmd/gdbforge/setup.go` | `InitB` wiring (`SetOnExecute`) |

---

## Related documentation

- [INPUT.md](INPUT.md) — interaction modes, key routing
- [ARCHITECTURE.md](ARCHITECTURE.md) — event bus, application wiring
- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) — onboarding, extension checklist

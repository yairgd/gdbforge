# Command System

cgdb-go routes user actions through a **hierarchical command tree**. Colon commands (`:window left`), tab completion, and key chords (`Ctrl+W h`) all resolve against the same `CommandNode` types, but through different entry points.

**Companion docs:** [INPUT.md](INPUT.md) · [ARCHITECTURE.md](ARCHITECTURE.md) · [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)

---

## Table of contents

- [Ownership model](#ownership-model)
- [CommandNode — the tree](#commandnode--the-tree)
- [CommandRegistry — owns the tree](#commandregistry--owns-the-tree)
- [CommandParser — navigates the tree](#commandparser--navigates-the-tree)
- [DSL — building the tree](#dsl--building-the-tree)
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
        LogW["LoggerWidget"]
        CmdW --> Parser
        CmdW -->|"Publish CompletionMsg"| Bus
        Bus -->|"Subscribe"| LogW
    end

    Parser -->|"references"| Registry
    DSL["ExapData() · Group/Cmd"] -->|"populates"| Tree
```

| Type | Role | Owns / holds |
|------|------|----------------|
| **`CommandNode`** | One node in the hierarchy | `Name`, `Children` trie, optional `Action` |
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
│   ├── down      → Action: OnFocusDown
│   └── break
│       ├── file  → Action: BreakFile
│       └── delete → Action: DeleteBreakpoint
├── break
│   ├── file      → Action: BreakFile
│   └── delete    → Action: DeleteBreakpoint
├── info
│   ├── registers → Action: ShowRegisters
│   └── threads   → Action: ShowThreads
├── run
│   ├── start
│   └── stop
├── edit
│   ├── about     → Action: showBuiltin("about")
│   └── gdb       → Action: showBuiltin("gdb")
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
| Container | `nil` | non-empty | `window`, `break`, `info` |
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
| `Group(name, children...)` | Container node with children attached |
| `(n *CommandNode).Group(name, children...)` | Inserts a group into `n`, returns `n` for chaining |

### Example (`DebuggerApp.ExapData` in `cmd/cgdb/command_tree.go`)

```go
func (a *DebuggerApp) ExapData() {
    a.commandReg.Root.
        Group("window",
            commands.Cmd("left", a.OnFocusLeft),
            commands.Cmd("right", a.OnFocusRight),
            commands.Cmd("up", a.OnFocusUp),
            commands.Cmd("down", a.OnFocusDown),
            commands.Group("break",
                commands.Cmd("file", a.BreakFile),
                commands.Cmd("delete", a.DeleteBreakpoint),
            ),
        ).
        Group("break",
            commands.Cmd("file", a.BreakFile),
            commands.Cmd("delete", a.DeleteBreakpoint),
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

## CmdWidget integration

`CmdWidget` (`internal/termui/cmd_widget.go`) owns a `CommandParser` created from the app's `CommandRegistry`.

```mermaid
sequenceDiagram
    participant User
    participant CmdW as CmdWidget
    participant Parser as CommandParser
    participant Bus as platform.EventBus
    participant LogW as LoggerWidget
    participant Tree as CommandNode tree

    User->>CmdW: Tab
    CmdW->>Parser: Sync(text, cursor)
    Parser->>Tree: replay tokens, Accept on spaces
    CmdW->>Parser: Suggestions()
    Parser->>Tree: current.Complete(token)
    Tree-->>Parser: []*CommandNode
    Parser-->>CmdW: suggestions
    CmdW->>Bus: Publish(CompletionMsg)
    Bus->>LogW: showCompletion → log.Info

    User->>CmdW: Enter
    CmdW->>Parser: Parse(line)
    Parser->>Tree: Accept each token
    CmdW->>Parser: Execute()
    Parser->>Tree: current.Action()
```

Wiring in `cmd/cgdb/setup.go`:

```go
a.cmdWidget = termui.NewCmdWidget(a.commandReg)
a.cmdWidget.Ctx = a.ctx   // provides EventBus for CompletionMsg
a.cmdWidget.Events = a.Events()

l := widgets.NewLoggerWidget(a.ctx) // Subscribe(ctx.Bus, showCompletion)
```

On **Enter**, the widget calls `Parse` then `Execute` if `CanExecute()`. Leaf actions run directly on the `CommandNode` — no `CommandID` / `SubmitMsg` indirection for tree commands.

---

## Tab completion via EventBus

Tab completion is announced as a **`termui.CompletionMsg`** on **`platform.EventBus`** (not an injected presenter):

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
| Subscriber | `LoggerWidget` | `platform.Subscribe` → `log.Info("completions: …")` (sink → viewport) |
| Future popup | new subscriber | Same `CompletionMsg`; no `CmdWidget` changes |

Producers depend only on the bus + message type. Consumers register independently (avoids constructor injection and cyclic wiring).

---

## Key bindings

Key chords use the **same `CommandNode` type** but a **different registry**:

| Entry point | Registry | Lookup |
|-------------|----------|--------|
| Colon commands | `CommandRegistry.Root` | `CommandParser` walks name tokens |
| Key chords | `KeyBindingRegistry` trie | `SearchPartial(key)` per keypress |

```go
// cmd/cgdb/keybindings.go
func (a *DebuggerApp) InitKeyBindings() {
    a.keyBindings = commands.NewKeyBindingRegistry()
    a.keyBindings.Bind(
        commands.NewCommand("move-left", func(args ...any) { a.OnFocusLeft() }),
        "<C-w>l", "<C-w><Left>",
    )
    // ...
}
```

A key binding can invoke the same handler as a colon command (`OnFocusLeft`) without sharing the parser — both are wired at init time in the application.

---

## Adding commands

### Colon command (tree)

1. Add a `Cmd` or nested `Group` in `ExapData()` (`cmd/cgdb/command_tree.go`).
2. Implement the handler on `DebuggerApp` in `cmd/cgdb/actions.go`.
3. No `CommandID` or `HandleCoreEvents` wiring needed for tree leaves — `Execute()` calls `Action` directly.

### Key chord

1. Add `a.keyBindings.Bind(…)` in `cmd/cgdb/keybindings.go`.

### Tab completion feedback

1. Subscribe to `termui.CompletionMsg` on `platform.EventBus` (see `LoggerWidget`), or add another subscriber for a popup UI.

---

## Package layout

| Path | Responsibility |
|------|----------------|
| `internal/commands/command_node.go` | `CommandNode`, `CommandRegistry` |
| `internal/commands/command_parser.go` | `CommandParser` — navigation, completion, execution |
| `internal/commands/dsl.go` | `Cmd`, `Group` builders |
| `internal/commands/key_binding_gegistry.go` | `KeyBindingRegistry` |
| `internal/termui/cmd_widget.go` | `:` input, parser sync, tab/enter, publishes `CompletionMsg` |
| `internal/termui/event.go` | `CompletionMsg` and other UI events |
| `internal/platform/event_bus.go` | Typed `Subscribe` / `Publish` |
| `internal/termui/logger_widget.go` | Completions subscriber + log sink |
| `cmd/cgdb/command_tree.go` | `ExapData` DSL |
| `cmd/cgdb/keybindings.go` | `InitKeyBindings` |
| `cmd/cgdb/actions.go` | Command action methods |
| `cmd/cgdb/setup.go` | `InitB` wiring |

---

## Related documentation

- [INPUT.md](INPUT.md) — interaction modes, key routing
- [ARCHITECTURE.md](ARCHITECTURE.md) — event bus, application wiring
- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) — onboarding, extension checklist

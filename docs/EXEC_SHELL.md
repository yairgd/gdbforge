# Exec / shell panes (`:!`)

cgdb-go can open an **external PTY session** in the focused pane, similar to Vim’s `:!` but as a persistent console widget (not a one-shot filter).

**Companion docs:** [COMMAND_SYSTEM.md](COMMAND_SYSTEM.md) · [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [INPUT.md](INPUT.md) · [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md)

---

## User commands

| Command | Effect |
|---------|--------|
| `:!bash` | Start `bash` on a PTY; show **Exec** widget in the focused pane |
| `:!ls` | Same for `ls` (short-lived; after exit, **any key** returns to the previous widget) |
| `:!ssh user@host` | Same for any argv |
| `:b exec` | Re-show the last Exec widget (if still registered) |
| `:b gdb` / `:b about` / `:b logger` / `:b breakpoint` / `:b threads` / `:b callstack` / `:b output` | Swap other built-in views into the focused pane |
| `:edit` / `:edit file.c` | Project source picker, or open a source file (`:e` = unique prefix) |
| `:b file.c` | Switch to an already-open file buffer |
| `<C-o>` (normal mode) | Jump back to the **previous widget** in this pane (Vim-style jump list) |

Bang may be glued or spaced: `:!ls` and `:! ls` both work.

---

## Architecture

```mermaid
flowchart LR
  Cmd[":!bash"] --> OnRun
  OnRun --> Client["execcli.ExecClient embeds ptyx.Client"]
  OnRun --> Widget["ExecWidget + ConsolePane"]
  Client -->|PtyOutputMsg| Bridge["EventInterrupt ExecOutputMsg"]
  Bridge --> Widget
  Widget -->|Send / SendRaw| Client
  OnRun --> Jump["push previous widget"]
  JumpBack["Ctrl-O JumpBack"] --> Jump
```

| Layer | Package / type | Role |
|-------|----------------|------|
| Command | `LeafRest("!", OnRun)` | Rest-args leaf; remainder of line → argv |
| PTY | `internal/ptyx.Client` | Shared PTY: mutex writes, `Subscribe` fan-out, `SetSize` |
| Client | `internal/execcli.ExecClient` | Thin wrapper (`*ptyx.Client` + initial winsize) |
| Event | `core.ExecOutputMsg` | UI-routed PTY chunks to the UI thread |
| Widget | `widgets.ExecWidget` | Line-oriented ConsolePane + live prompt + ANSI |
| App | `DebuggerApp.OnRun` | Create client/widget, `swapFocusedWidget`, insert mode |

GDB uses the same `ptyx.Client` via `gdb.GDBClient`. Exec reuses **ConsolePane** but has **no MI parser** — plain text + ANSI.

---

## Rest-args (`LeafRest` / `RestArgs`)

Normal colon commands walk **every** token as a tree child (`:set equalalways`). That cannot parse `:!ssh root@host`.

A **rest-args** leaf (`RestArgs == true`) means: after accepting this node, **stop walking the tree** and pass remaining tokens to `Action`.

```text
:!ssh root@host
  │ └──────────┘
  │    p.args  (not Accept()'d as children)
  └─ current stays on "!" node → OnRun
```

Implementation: `internal/commands/dsl.go` (`CmdRest` / `LeafRest`) and `CommandParser.Parse` / `Sync` (including glued `:!ls`).

---

## ConsolePane live prompt

For both GDB and Exec, the input caret can sit on the **same row** as the last buffer line when that line is marked live (`SetLivePrompt` / `EnsureLivePrompt`):

- **GDB:** on MI `(gdb)` ready → `EnsureLivePrompt()` with `"(gdb) "`
- **Exec:** incomplete PTY line (bash PS1) → `SetLivePrompt(true)` while pending

When `livePrompt` is set and follow-tail is on, `ConsolePane.Draw` paints the host line (ANSI-aware) and draws `InputLine` immediately after its visible width.

---

## ANSI rendering

Exec enables `ConsolePane.SetANSI(true)`. Scrollback uses `DrawANSIText` / `StripANSI` in `internal/termui/utf.go`:

- SGR colors (`\x1b[01;32m` …) → tcell styles
- OSC / private CSI (title, bracketed paste) → skipped, not drawn
- Copy selection strips ANSI so the clipboard is plain text

---

## Copy / paste

| Action | Behavior |
|--------|----------|
| Mouse drag + `Ctrl-C` | Copy selection from scrollback (ANSI stripped) |
| `Ctrl-V` | Paste into the **input line** (viewport is read-only) |
| `EventClipboard` | Forwarded by `TermApp` to focused widgets |

---

## Jump list (`Ctrl-O`)

`DebuggerApp.swapFocusedWidget` pushes the outgoing widget before `:b` / `:e` / `:!` swaps.

| API | Role |
|-----|------|
| `pushWidgetJump` | Append (dedupe consecutive, cap 32) |
| `JumpBack` | Pop and `ReplaceFocusedWidget` without pushing |
| Binding | `<C-o>` in `cmd/cgdb/keybindings.go` (normal mode) |

Example: GDB → `:b about` → `<C-o>` → GDB again.

---

## Lifecycle notes

- Each `:!…` **restarts** the exec session (closes previous `ExecClient`).
- When the PTY process exits (or Ctrl-D closes it), the Exec pane shows  
  `[exec] process exited — press any key to return`, then **any key** runs `JumpBack` to the previous widget and clears the `exec` builtin.
- App exit / last-pane `:quit` still closes `execClient` if present (`DebuggerApp.Close`, `Quit`).

---

## Key source files

| Path | Responsibility |
|------|----------------|
| `cmd/cgdb/command_tree.go` | `LeafRest("!", a.OnRun)` |
| `cmd/cgdb/actions.go` | `OnRun` |
| `cmd/cgdb/builtins.go` | `swapFocusedWidget`, `JumpBack` |
| `cmd/cgdb/keybindings.go` | `<C-o>` |
| `internal/execcli/exec_client.go` | PTY process I/O |
| `internal/cgdb/widgets/exec_widget.go` | UI adapter |
| `internal/commands/{dsl,command_parser,command_node}.go` | Rest-args |
| `internal/termui/console_pane.go` | Live prompt + paste into input |
| `internal/termui/utf.go` | ANSI draw / strip |
| `internal/core/events.go` | `ExecOutputMsg` |

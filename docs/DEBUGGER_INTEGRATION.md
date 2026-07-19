# Debugger Integration

cgdb-go connects to debug targets through **backend adapters** that implement `core.Session` (`Debugger` + lifetime + PTY mux). The first adapter is **GDB MI2 over a shared PTY** (`ptyx`). The same session is used by the GDB console, in-app `:AI`, and future MCP/REST frontends.

**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) · [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [EXEC_SHELL.md](EXEC_SHELL.md) · [PLUGINS.md](PLUGINS.md)

---

## Table of contents

- [Integration overview](#integration-overview)
- [Debugger interface](#debugger-interface)
- [PTY mux](#pty-mux)
- [Breakpoints and source sync](#breakpoints-and-source-sync)
- [GDB integration](#gdb-integration)
- [GdbMcpService and in-app AI](#gdbmcpservice-and-in-app-ai)
- [MI2 parsing pipeline](#mi2-parsing-pipeline)
- [GDBWidget bridge](#gdbwidget-bridge)
- [Future OpenOCD integration](#future-openocd-integration)
- [Future JTAG integration](#future-jtag-integration)
- [Future kernel debugging](#future-kernel-debugging)
- [Design constraints](#design-constraints)

---

## Integration overview

Application data flows **Service → Event Bus → Model → Widget** (target). Today the GDB path is still widget-local via `EventInterrupt`, but ownership and the external API are session-based.

```mermaid
flowchart TB
    subgraph UI["UI · termui / widgets"]
        GDBW["GDBWidget owns client"]
        Cons["ConsolePane"]
    end

    subgraph App["Application · cmd/cgdb"]
        AI[":AI OnAI"]
        MCP["GdbMcpService"]
    end

    subgraph Domain["Domain · core"]
        SessIF["Session / Debugger / PTYWriter"]
        PtyMsg["PtyOutputMsg"]
        UIMsg["GdbOutputMsg · ExecOutputMsg"]
    end

    subgraph PTYLayer["PTY · ptyx"]
        Pty["ptyx.Client"]
    end

    subgraph Backend["gdb"]
        Client["GDBClient embeds ptyx"]
        MI["GdbInputState · MiUpdate"]
    end

    subgraph External["External"]
        GDB["GDB --interpreter=mi2"]
        LLM["Claude / OpenAI API"]
    end

    GDBW --> Cons
    GDBW -->|"owns"| Client
    Client --> Pty
    MCP -->|"Session only"| SessIF
    AI --> MCP
    AI --> LLM
    SessIF --> Client
    Pty -->|"Subscribe fan-out"| PtyMsg
    PtyMsg -->|"UI bridge"| UIMsg
    UIMsg --> GDBW
    Pty --> GDB
    GDBW --> MI
```

**Dependency rules:**

- `internal/gdb` and `internal/ptyx` must not import `internal/termui`
- `GDBWidget` owns the concrete `GDBClient`; app/MCP use `core.Session` via `app.GDB()` / `gdbWidget.Session()`
- Never `Close()` the session from MCP/AI while the widget owns it

---

## Debugger interface

```go
type Debugger interface {
    Send(cmd string) error
    SendRaw(raw string) error
}

type PTYWriter interface {
    Send(cmd string) error
    SendRaw(raw string) error
}

// Session: lifetime + exclusive write + shared read.
type Session interface {
    Debugger
    Close()
    Subscribe() (ch <-chan PtyOutputMsg, cancel func())
    WithWrite(ctx context.Context, fn func(w PTYWriter) error) error
}
```

Minimal by design — sends commands to the backend. Responses arrive asynchronously via `Subscribe` / UI events, not as return values from `Send`.

**Design rationale:** MI2 and GDB CLI are streaming protocols. Blocking `Send` → response would deadlock when async `*stopped` records arrive mid-command. `GdbMcpService.GdbCommand` adds a **windowed capture** on a private subscription for tool results (best-effort until tokenized MI waiters).

**Ownership:** `GDBWidget` creates and owns the `GDBClient` (`NewGDBWidget` → `Start` → `Close`). `DebuggerApp` holds `gdbMcp` as a peer constructed from `gdbWidget.Session()`. External APIs use `app.GDB() core.Session`.

Future extensions (separate interfaces):

| Interface | Purpose |
|-----------|---------|
| `BreakpointManager` | Add/remove/list breakpoints |
| `RegisterReader` | Read register sets |
| `MemoryReader` | Read/write memory |

These will emit `core.Event` updates rather than synchronous returns.

---

## PTY mux

One `ptmx` (`ptyx.Client`). Two rules:

| Direction | Rule |
|-----------|------|
| **Write** | **Exclusive** — `WithWrite` / `Send` / `SendRaw` share one mutex; only one of UI / MCP / App holds it at a time |
| **Read** | **Shared** — every `Subscribe()` channel receives the same chunks |

Writers (`PTYOwner` on `AppState`):

| Owner | Who | Console paint |
|-------|-----|---------------|
| `ui` | GDB / Exec console submit | Yes |
| `mcp` | `:AI` / `GdbCommand` | Suppressed in GDBWidget |
| `app` | Silent MI / App writes: `-break-list`, file list, **CodeWidget Space**, **BreakpointWidget e/d**, stop-driven thread/stack Query | Not suppressed (MI `^done` rarely paints; must not hide UI `n` / stop console) |

```mermaid
flowchart LR
  UI["GDBWidget PTYOwnerUI"]
  MCP["GdbMcpService PTYOwnerMCP"]
  App["App / Code / BP widget PTYOwnerApp"]
  Lock["write lock"]
  UI --> Lock
  MCP --> Lock
  App --> Lock
  Lock --> PTMX["ptmx"]
  PTMX --> Fan["broadcast"]
  Fan --> ChUI["UI Subscribe"]
  Fan --> ChMCP["MCP Subscribe"]
```

GDB and exec (`:!`) both embed `*ptyx.Client`. UI bridges convert `PtyOutputMsg` → `GdbOutputMsg` / `ExecOutputMsg` for interrupt routing.

**Session model on AppState:** `SourceFiles` (filled once from `-file-list-exec-source-files` when empty, e.g. first stop / `:edit`), `CurrentFile` / `CurrentLine` (updated on `*stopped`), `MarkColor` (file-picker selection; `:set markcolor`), `BreakColor` / `BreakDisabledColor` (enabled/disabled BP backgrounds; `:set breakcolor` / `:set breakdisabledcolor`), `EscToCode` (Esc focuses CodeWidget; `:set esctocode` / `:set noesctocode`; default **on**), `ContinueAfterClear`. Each open source file has its own CodeWidget (`:edit name`); `:b filename` switches among open file buffers and builtins. `:edit` opens a FileListWidget of project sources. Breakpoint gutters sync via `=breakpoint-*` / Space hooks → coalesced `-break-list` (not re-painted from a stale list on every stop).

---

## Breakpoints and source sync

Breakpoints are coordinated across the GDB console, CodeWidget, BreakpointWidget, and MCP. GDB/MCP notifies publish **`BreakpointsChangedMsg`** on `platform.EventBus`; `DebuggerApp` Subscribes and refreshes from that event (no sleep/timer debounce).

## Breakpoints while the inferior is running

While the program is in `continue` / `^running`, sync GDB does not process a queued `break` until the target stops. Space (and BreakpointWidget e/`d`) therefore:

1. Send Ctrl-C (`\x03`) to interrupt
2. Send `break` / `clear` / `-break-delete`
3. On **insert** (`break`): send `continue` so execution can hit the new breakpoint
4. On **remove** (`clear` / `-break-delete`): send `continue` only if `:set continueafterclear` (default **off** — stay stopped)

`AppState.InferiorRunning` tracks `^running` → `*stopped` for this path. `AppState.ContinueAfterClear` is toggled with `:set continueafterclear` / `:set nocontinueafterclear`.

### Builtins and keys

| Surface | How to open | Keys |
|---------|-------------|------|
| **BreakpointWidget** | `:b breakpoint` (default pane) | `j`/`k` or Up/Down — bold selection; **`e`** — toggle enable/disable; `d` — delete; rows use AppState break colors (red/yellow bg) |
| **OutputWidget** | `:b output` (default pane, top-right) | `j`/`k` / PgUp/PgDn — scroll; `<C-l>` clear |
| **ThreadWidget** | `:b threads` (default pane) | `j`/`k` or Up/Down — bold selection; filled on stop |
| **CallStackWidget** | `:b callstack` (default pane) | `j`/`k` or Up/Down — bold selection; filled on stop |
| **FileListWidget** | `:edit` | `j`/`k` or Up/Down — mark color from `:set markcolor`; Enter opens; mouse: first click selects, second click on marked row opens CodeWidget |
| **CodeWidget** | `:edit name` / stop / `:b file` | Up/Down or `j`/`k` — bold cursor line; **Space** — insert/remove break; **`e`** — enable/disable (yellow gutter when disabled; same as BreakpointWidget `e`) |

Empty Breakpoint list shows `no breakpoints`. Otherwise each row is breakpoint info only (no column header), e.g. `1  y  hello.c:23`. Disabled rows are gray (`n`).

### Ownership of the list

`BreakpointWidget` owns an **internal list**:

| Action | Internal list | GDB | CodeWidget red marks |
|--------|---------------|-----|----------------------|
| **e** while enabled | Row stays, `Enabled=false` | `-break-delete` (removed from GDB) | Cleared for that line |
| **e** while disabled | Row stays, `Enabled=true` | `break file:line` (re-inserted) | Restored |
| **d** | Row removed | Deleted if it was in GDB | Cleared |
| External `b` / Space / MCP | Merged via `MergeFromGDB` | As GDB reports | Enabled rows only |

Disabled rows are **kept** across `-break-list` refresh (they are intentionally absent from GDB). `MergeFromGDB` updates numbers for live breakpoints and drops enabled rows that vanished elsewhere.

### Callback chain

Wired in `cmd/cgdb/builtins.go`:

| Hook | Handler |
|------|---------|
| `GDBWidget.SetOnBreakpointsChanged` | `onBreakpointsChanged` → `Publish(BreakpointsChangedMsg)` |
| `GdbMcpService.OnBreakpointsChanged` | same |
| `EventBus` Subscribe | `onBreakpointsChangedMsg` → coalesced `-break-list` |
| `BreakpointWidget.OnChange` | `onBreakpointListChanged` → `paintCodeBreakmarks` |

```mermaid
flowchart TD
  MI["MI =breakpoint-created/deleted/modified"] --> GW["GDBWidget.onBreakpoints"]
  MCP["MCP GdbCommand"] --> MCPCB["OnBreakpointsChanged"]
  GW --> OBC["onBreakpointsChanged"]
  MCPCB --> OBC
  OBC --> BUS["Publish BreakpointsChangedMsg"]
  BUS --> SUB["Subscribe → coalesce -break-list"]
  SUB --> MERGE["bpWidget.MergeFromGDB"]
  BP["bpWidget e/d"] --> OC["bpWidget.OnChange"]
  MERGE --> OC
  OC --> PAINT["onBreakpointListChanged → paintCodeBreakmarks"]
  PAINT --> CW["CodeWidget.SetBreakInfos"]
  CODE["CodeWidget Space"] --> PTY["PTYOwnerApp break/clear"]
  PTY --> MI
```

`DebuggerApp` Subscribes to `BreakpointsChangedMsg` in `initBuiltins` and runs a coalesced `-break-list`:

| State | Behavior |
|-------|----------|
| Idle | First publish starts a background `runBreakpointRefresh` |
| Refresh in flight | Further publishes set `bpRefreshPending` only |
| After refresh | If pending → one trailing `-break-list`; else clear running flag |

No `time.Sleep` debounce — coalesce is event-driven (pending flag). Redraw uses `PostEvent(breakpointsUIMsg)` on the UI thread. CodeWidget red marks still come from `OnChange` → `paintCodeBreakmarks` → `SetBreakInfos` (not a separate bus subscriber on the code panes).

Only `=breakpoint-created` / `=breakpoint-deleted` trigger a `-break-list` refresh (not `=breakpoint-modified` hit-count updates during `n`/`continue`).

### CodeWidget details

- Syntax highlight via Chroma (`terminal256`); line numbers; breakpoint lines use a red number background.
- PC line: `━━▶` + dark slate row background.
- Cursor line (when focused): bold + dark blue (unless it is the PC line).
- Space uses basename locations (`break hello.c:23` / `clear hello.c:23`) under `PTYOwnerApp`.
- Horizontal scroll in ANSI mode uses visible columns (not raw byte offsets) so panes stay readable after `:vs`.

PTY exclusivity remains `ptyx.WithWrite`; `PTYOwner` tells GDBWidget when to suppress console paint (**MCP only** — App stop Queries must not hide UI console output).

### Threads and call stack on stop

On each qualifying `*stopped`, `DebuggerApp` coalesces (pending flag, no timer):

1. `Query("-thread-info")` → `ParseThreadInfo` → `ThreadWidget.SetItems`
2. `Query("-stack-list-frames")` → `ParseStackListFrames` → `CallStackWidget.SetItems`
3. `PostEvent(debugInfoUIMsg)` → `RequestFrame`

Independent of `BreakpointsChangedMsg` (BP marks stay on breakpoint-change events).

---

## GDB integration

### Client startup

`gdb.NewGDBClient()` wraps `ptyx.Client` (`gdb_client.go` / `ptyx/client.go`):

1. Builds `gdb --interpreter=mi2 <target>` argv.
2. `ptyx.New` — PTY, raw mode, reader fan-out.
3. Sends initial newline to trigger prompt.

```go
argv := []string{"gdb", "--interpreter=mi2", "hello"}
pty, err := ptyx.New(argv, ptyx.Options{})
```

**Current limitation:** reply correlation (tokenized MI → waiter) is not built yet — `GdbCommand` uses idle/max window capture on the raw stream.

### Send paths

| Method | Use |
|--------|-----|
| `Send(cmd)` | Append `\n`, send CLI/MI command (takes write lock) |
| `SendRaw(raw)` | Send raw bytes (SIGINT, …) under write lock |
| `WithWrite(ctx, fn)` | Hold write lock for multi-step MCP capture |

### Output path

```go
ch, cancel := client.Subscribe()
defer cancel()
for msg := range ch {
    // UI posts EventInterrupt(GdbOutputMsg); MCP/AI parse the same stream
}
```

`Close()` (or `cancel`) closes subscription channels → `GDBWidget` receives `"gdb-exit"` when its subscription ends.

---

## GdbMcpService and in-app AI

Same process as the TUI (required for one debug context).

| Piece | Role |
|-------|------|
| `internal/mcp/gdb_service.go` | Tool core: `GdbCommand` under `WithWrite` + Subscribe capture |
| `internal/mcp/agent.go` | LLM tool loop (Anthropic or OpenAI) |
| `:AI` / `:ai` | Rest-args colon command → `OnAI` → `Ask` in a goroutine |

```text
:AI why is there a memory leak
  → GdbMcpService.Ask
  → LLM may call gdb_command("info leak") / "b main" / …
  → same Session as the GDB pane (exclusive write, shared read)
  → answer appended to GDB console
```

**Env:** `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`; optional `CGDB_AI_MODEL`.

**Do not** use stdio MCP inside the TUI process (keyboard owns stdin). Optional TCP MCP for external hosts can expose the same tools later; in-app `:AI` is the primary door.

---

## MI2 parsing pipeline

GDB MI2 emits several record types:

| Prefix | Type | Example |
|--------|------|---------|
| `^` | Result | `^done`, `^error`, `^running` |
| `~` | Console stream | `~"Hello\n"` |
| `@` | Target stream | `@"program output\n"` |
| `&` | Log stream | `&"warning\n"` |
| `*` | Exec async | `*stopped,reason="breakpoint-hit"` |
| `=` | Notify async | `=breakpoint-created,...` |
| `(gdb)` | Prompt | Ready for input |

### GdbInputState (streaming)

`PushRaw` is called on the **UI thread** for each coalesced `GdbOutputMsg` chunk:

1. Append bytes to `lineBuf`; split on `\n`.
2. For each complete line, classify the MI record and accumulate an `MiUpdate`.
3. Return immediately — no timer, no wait for a “full burst”.

| Record | Effect on `MiUpdate` |
|--------|----------------------|
| `~` / `@` | Decode stream payload → `DisplayLines` |
| `&` | Ignored for display (CLI echo; UI already echoes submits) |
| `^error` | Set `Error` state; surface `msg=` |
| `^done` / `^running` / … | Update `GdbState` |
| `*stopped` | Fill `Stopped` (`reason`, `thread-id`) |
| `(gdb)` | `PromptReady` |

Incomplete lines remain in `lineBuf` across chunks.

**Design rationale:** GDB often splits writes mid-line; newline buffering is required. Per-line dispatch is enough for correctness and feels faster than a 100ms debounce.

### MiMsg (batch helper)

`MiMsg` / `NewMiMsg` / `CreateBufferForLine` remain as a batch-oriented helper for offline or test parsing. The live GDB pane uses streaming `MiUpdate` from `GdbInputState`.

```go
type MiUpdate struct {
    DisplayLines []string
    PromptReady  bool
    State        GdbState
    ErrorMsg     string
    Stopped      *MiStopMsg
}
```

### MI string decoding

`DecodeMIString` handles GDB's C-style escapes including **octal UTF-8 sequences** (`\342\235\214`). `ExpandTabs` expands tab stops for aligned output.

Implementation: `mi.go`, `mi_msg.go`, `mi_state.go`.

---

## GDBWidget bridge

`GDBWidget` owns the GDB session and is a thin adapter over shared termui REPL pieces:

```text
InputLine  →  ConsolePane  →  GDBWidget  →  GDBClient (owned) → ptyx
(edit/hist)   (scrollback +     (create/Start/Close,
               walking prompt)   MI + Send)
```

`NewGDBWidget(gdbPath, prog, args...)` spawns the client; `Start(screen)` bridges `Subscribe` → coalesced `EventInterrupt(GdbOutputMsg)` (~16ms / 64KiB batches so free-running stdout does not flood the UI thread); `Close()` tears the process down. Presentation is a **native GDB session** (`(gdb) b main` then raw console output) — not labeled chat (`user:` / `gdb:`).

Painting stays on the tcell event loop (tcell is not thread-safe). The bridge goroutine only batches bytes before `PostEvent`.

```mermaid
sequenceDiagram
    participant User
    participant GDBW as GDBWidget
    participant Cons as ConsolePane
    participant State as GdbInputState
    participant Client as GDBClient
    participant GDB as GDB

    User->>Cons: type / Enter
    Cons->>GDBW: OnSubmit(cmd)
    GDBW->>Cons: EchoSubmit(prompt+cmd)
    GDBW->>Client: Send(cmd)
    Client->>GDB: PTY write
    GDB-->>Client: MI output chunk
    Client-->>GDBW: EventInterrupt GdbOutputMsg
    GDBW->>State: PushRaw(chunk)
    State-->>GDBW: MiUpdate
    GDBW->>Cons: AppendLines / Draw
```

| Component | File | Role |
|-----------|------|------|
| `InputLine` | `termui/input_line.go` | Text, cursor, readline history/editing |
| `ConsolePane` | `termui/console_pane.go` | Scrollback Viewport, walking prompt, `EchoSubmit`, key shell |
| `GDBWidget` | `cgdb/widgets/gdb_widget.go` | Owns client; `OnSubmit` → Send; MI → AppendLines |
| `GdbInputState` | `gdb/mi_state.go` | Stream `PushRaw` → `MiUpdate` |
| `ptyx.Client` | `ptyx/client.go` | Shared PTY mux |

### Console layout (walking prompt)

Terminal-style after `Ctrl+L` (clear / screen reset) — owned by `termui.ConsolePane`:

1. Empty scrollback → `(gdb)` + caret at **top-left**.
2. Each new output line → prompt moves **one row down**.
3. While free rows remain → leave blank space below; do **not** jump the prompt to the bottom.
4. When the pane is full → pin prompt to the last row and scroll the viewport (`followTail`).

While the user scrolls history (`followTail` off), the prompt stays on the bottom row.

Echo is `prompt+cmd` only (native GDB session, not chat labels).

Draw highlights:

- Lines starting with `>>>` — teal bold (future: stop reason).
- Echoed / prompt text with `(gdb)` — yellow.

---

## Future OpenOCD integration

[OpenOCD](https://openocd.org/) exposes a **Telnet command port** and **TCL scripting** for embedded targets.

Planned adapter: `internal/openocd` (not yet created).

| Aspect | Plan |
|--------|------|
| Transport | TCP telnet or pipe to `openocd` process |
| Protocol | TCL commands + event text (not MI2) |
| Interface | Same `core.Session` for send/subscribe; adapter translates |
| UI impact | None — new backend package only |

```mermaid
flowchart LR
    UI["termui"]
    Core["core.Session"]
    GDB["gdb.GDBClient"]
    OOCD["openocd.Client · planned"]

    UI --> Core
    Core --> GDB
    Core --> OOCD
```

**Design decision:** OpenOCD is a **separate backend**, not a GDB wrapper. Some workflows may use both (OpenOCD for flash/reset, GDB for symbols) — session orchestration belongs in app/`core`.

---

## Future JTAG integration

JTAG debugging may arrive through:

1. **OpenOCD** as transport (preferred — reuse OpenOCD adapter).
2. Direct **JTAG library** (e.g., libftdi) for specialized hardware bring-up.

cgdb-go UI would expose:

- Chain scan / device selection pane.
- TAP state indicator.
- DR/IR scan views.

These are **feature panes** ([PLUGINS.md](PLUGINS.md)), not core UI changes.

---

## Future kernel debugging

Kernel debugging introduces unique requirements:

| Requirement | UI response |
|-------------|-------------|
| Multiple address spaces | Memory pane with context selector |
| Crash dumps / vmcore | Read-only source + backtrace panes |
| Remote targets | Backend connection manager (not UI) |
| Module / symbol load | Async events → source pane refresh |

Planned backend options:

- GDB remote (`target remote :1234`)
- `crash` utility integration for dump analysis
- Custom `/proc/kcore` readers

**Design constraint:** kernel workflows must not fork the UI. New panes and backends extend the existing widget and event model.

---

## Design constraints

| Constraint | Reason |
|------------|--------|
| Backends never import `termui` | Testability, headless automation |
| Async-only responses | MI2 / OpenOCD are streaming |
| Exclusive PTY write / shared read | UI + AI share one `ptmx` safely |
| Parse MI in gdb layer | Widgets display buffers, not raw protocol |
| Session config outside MCP | Target binary/args from CLI; AI uses live session |
| Same-process AI | One debug context for manual + `:AI` |

---

## Related documentation

- [EXEC_SHELL.md](EXEC_SHELL.md) — `:!` exec panes (same `ptyx`)
- [INPUT.md](INPUT.md) — GDB key forwarding
- [PLUGINS.md](PLUGINS.md) — custom debugger panes
- [ROADMAP.md](ROADMAP.md) — backend milestones
- [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md) — package map

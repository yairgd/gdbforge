# Debugger Integration

NewCGDB connects to debug targets through **backend adapters** that implement `core.Debugger`. The first adapter is **GDB MI2 over a PTY**. Future adapters will cover **OpenOCD**, **JTAG**, and **kernel debugging** workflows.

**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) · [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [PLUGINS.md](PLUGINS.md)

---

## Table of contents

- [Integration overview](#integration-overview)
- [Debugger interface](#debugger-interface)
- [GDB integration](#gdb-integration)
- [MI2 parsing pipeline](#mi2-parsing-pipeline)
- [GDBWidget bridge](#gdbwidget-bridge)
- [Future OpenOCD integration](#future-openocd-integration)
- [Future JTAG integration](#future-jtag-integration)
- [Future kernel debugging](#future-kernel-debugging)
- [Design constraints](#design-constraints)

---

## Integration overview

```mermaid
flowchart TB
    subgraph UI["UI · termui"]
        GDBWidget["GDBWidget"]
        Console["Console panes"]
    end

    subgraph Domain["Domain · core"]
        DebuggerIF["Debugger interface"]
        Buffer["Buffer / Viewport"]
        Events["GdbOutputMsg"]
    end

    subgraph Backend["Backend · gdb"]
        Client["GDBClient"]
        MI["MiMsg / GdbInputState"]
        PTY["PTY I/O"]
    end

    subgraph External["External"]
        GDB["GDB --interpreter=mi2"]
        Target["Debug target"]
    end

    GDBWidget --> DebuggerIF
    GDBWidget --> Buffer
    DebuggerIF --> Client
    Client --> PTY --> GDB --> Target
    Client --> Events --> GDBWidget
    Client --> MI
```

*Source: [`diagrams/debugger_integration.mermaid`](diagrams/debugger_integration.mermaid)*

**Dependency rule:** `internal/gdb` must not import `internal/termui`. UI widgets depend on `core.Debugger`, not on concrete GDB types (though `GDBWidget` currently also uses `gdb.GdbInputState` — a coupling to refactor behind an adapter).

---

## Debugger interface

```go
type Debugger interface {
    Send(cmd string) error
    SendRaw(raw string) error
}
```

Minimal by design — sends commands to the backend. Responses arrive asynchronously via channels/events, not as return values.

**Design rationale:** MI2 and GDB CLI are streaming protocols. Blocking `Send` → response would deadlock when async `*stopped` records arrive mid-command. The interface reflects fire-and-forget command submission.

Future extensions (separate interfaces):

| Interface | Purpose |
|-----------|---------|
| `DebuggerSession` | Attach/detach, symbol loading |
| `BreakpointManager` | Add/remove/list breakpoints |
| `RegisterReader` | Read register sets |
| `MemoryReader` | Read/write memory |

These will emit `core.Event` updates rather than synchronous returns.

---

## GDB integration

### Client startup

`gdb.NewGDBClient()` (`gdb_client.go`):

1. Spawns `gdb --interpreter=mi2 <target>`.
2. Starts PTY via `creack/pty`.
3. Sets raw terminal mode on PTY (no echo, non-canonical).
4. Starts reader goroutine → `core.GdbOutputMsg` channel.
5. Sends initial newline to trigger prompt.

```go
cmd := exec.Command("gdb", "--interpreter=mi2", "hello")
ptmx, err := pty.Start(cmd)
```

**Current limitation:** target binary hardcoded as `"hello"`. Will move to session configuration.

### Send paths

| Method | Use |
|--------|-----|
| `Send(cmd)` | Append `\n`, send CLI/MI command |
| `SendRaw(raw)` | Send raw bytes (SIGINT, arrow keys) |

### Output path

Reader goroutine:

```go
for {
    n, err := c.ptmx.Read(buf)
    if n > 0 {
        output <- core.GdbOutputMsg{Data: string(data)}
    }
    // ...
}
```

Channel closes on EOF/error → `GDBWidget` receives `"gdb-exit"` interrupt.

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

### GdbInputState

Batches incoming lines with a **100ms debounce timer**:

1. `PushLine(line)` appends to burst buffer, resets timer.
2. On timer fire → widget processes batch via `NewMiMsg(buf)`.

**Design rationale:** GDB sends bursts of related lines (console + result + prompt). Debouncing produces atomic UI updates.

### MiMsg

`NewMiMsg` processes a line batch:

```go
type MiMsg struct {
    CmdLine      []string   // ~ console stream
    GdbLog       []string   // & log stream
    GdbError     []string   // ^error messages
    TargetStdOut []string   // @ target stream
    MiStopMsg    MiStopMsg  // *stopped details
    gdbState     GdbState   // Done / Error / Running
}
```

`CreateBufferForLine()` converts parsed state into display lines for `core.Buffer`.

### MI string decoding

`DecodeMIString` handles GDB's C-style escapes including **octal UTF-8 sequences** (`\342\235\214`). `ExpandTabs` expands tab stops for aligned output.

Implementation: `mi.go`, `mi_msg.go`, `mi_state.go`.

---

## GDBWidget bridge

`GDBWidget` connects the GDB client to the UI:

```mermaid
sequenceDiagram
    participant User
    participant Widget as GDBWidget
    participant State as GdbInputState
    participant MI as MiMsg
    participant Buf as core.Buffer
    participant Client as GDBClient
    participant GDB as GDB

    User->>Widget: KeyEnter / typed command
    Widget->>Client: Send(input)
    Client->>GDB: PTY write
    GDB-->>Client: MI output
    Client-->>Widget: EventInterrupt GdbOutputMsg
    Widget->>State: PushLine
    Note over State: timer 100ms
    State-->>Widget: timeout
    Widget->>MI: NewMiMsg(buffer)
    MI-->>Buf: CreateBufferForLine
    Widget->>Widget: Draw
```

Key components:

| Component | Role |
|-----------|------|
| `core.Buffer` | Scrollable output lines |
| `core.Viewport` | Visible window + follow-bottom |
| `InputBuf` / `Cursor` | User command editing |
| `handleAsyncRecord` | React to `*stopped` (stub) |

Draw highlights:

- Lines starting with `>>>` — teal bold (future: stop reason).
- Lines starting with `(gdb)` — yellow prompt.

---

## Future OpenOCD integration

[OpenOCD](https://openocd.org/) exposes a **Telnet command port** and **TCL scripting** for embedded targets.

Planned adapter: `internal/openocd` (not yet created).

| Aspect | Plan |
|--------|------|
| Transport | TCP telnet or pipe to `openocd` process |
| Protocol | TCL commands + event text (not MI2) |
| Interface | Same `core.Debugger` for send; adapter translates |
| UI impact | None — new backend package only |

```mermaid
flowchart LR
    UI["termui"]
    Core["core.Debugger"]
    GDB["gdb.GDBClient"]
    OOCD["openocd.Client · planned"]

    UI --> Core
    Core --> GDB
    Core --> OOCD
```

**Design decision:** OpenOCD is a **separate backend**, not a GDB wrapper. Some workflows may use both (OpenOCD for flash/reset, GDB for symbols) — session orchestration belongs in `internal/core/session.go`.

---

## Future JTAG integration

JTAG debugging may arrive through:

1. **OpenOCD** as transport (preferred — reuse OpenOCD adapter).
2. Direct **JTAG library** (e.g., libftdi) for specialized hardware bring-up.

NewCGDB UI would expose:

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
| Parse in backend layer | Widgets display buffers, not raw MI |
| Session config outside widgets | Target binary, ports, symbols in `core/session` |

---

## Related documentation

- [INPUT.md](INPUT.md) — GDB key forwarding
- [PLUGINS.md](PLUGINS.md) — custom debugger panes
- [ROADMAP.md](ROADMAP.md) — backend milestones
- [DIRECTORY_STRUCTURE.md](DIRECTORY_STRUCTURE.md) — package map

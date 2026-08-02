# PTY architecture — how the pieces talk

High-level view of **who holds which PTY end** (master vs slave), how the **GDB console** and the **under-debug program** get separate terminals, how **Delve** differs (PTY vs TCP), and how **`:b io`** / **external terminals** plug in.

For MI protocol, MCP, and backend details see [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md). For user recipes see [USER_GUIDE.md](USER_GUIDE.md) and [../lua/README.md](../lua/README.md).

---

## Why two channels?

A debugger session needs **two independent byte streams**:

| Channel | What travels | UI surface |
|---------|--------------|------------|
| **Debugger console** | GDB MI / Delve CLI, prompts, replies | `:b gdb` (`GDBWidget`) |
| **Inferior stdio** | The program’s stdin / stdout / stderr | `:b io` **or** an external terminal |

Mixing them on one PTY (classic “everything in the GDB TTY”) fights TUIs and makes Ctrl-C ambiguous. gdbforge therefore uses a **dual-PTY** model for local sessions (and TCP for preferred Go/Delve TUI bring-up).

---

## PTY vocabulary (master / slave)

A Linux PTY is a **pair**:

```text
                    ┌─────────────────┐
  Application  ←──► │  MASTER (/dev/ptmx side)  │  ← held by gdbforge or a terminal emulator
                    └────────┬────────┘
                             │ kernel couples bytes both ways
                    ┌────────▼────────┐
  Child process ←──► │  SLAVE  (/dev/pts/N)      │  ← GDB, Delve, or the inferior “sees” a tty
                    └─────────────────┘
```

| Role | Typical holder in gdbforge | Meaning |
|------|----------------------------|---------|
| **Master** | `ptyx.Client` or `ptyx.TTY` (or the external emulator) | Read “what the slave wrote”; write “what the slave should read as keyboard” |
| **Slave** | Debugger process or inferior | Looks like a real terminal (`isatty`, line discipline, window size) |

**Rule of thumb:** whoever should see the other side’s keystrokes/output holds the **master**. The process that believes it has a terminal opens (or is given) the **slave**.

---

## Big picture — components

```mermaid
flowchart TB
  subgraph UI["UI panes"]
    GDBW[":b gdb · GDBWidget"]
    IOW[":b io · OutputWidget"]
  end

  subgraph App["DebuggerApp bridges"]
    BridgeG["gdb_console.go"]
    BridgeI["io_console.go"]
    InfCtl["inferior_tty.go"]
  end

  subgraph Ptyx["internal/ptyx"]
    C["Client · debugger MASTER"]
    T["TTY · inferior MASTER"]
  end

  subgraph Backends["Backends"]
    GC["gdb.GDBClient"]
    DC["dlv.Client"]
  end

  subgraph Ext["Outside gdbforge"]
    Term["GDBFORGE_TERMINAL"]
    Hold["hold pts · sleep infinity"]
    Headless["dlv --headless --listen"]
  end

  GDBW --> BridgeG --> C
  IOW --> BridgeI --> T
  GC --> C
  GC --> T
  DC --> C
  DC --> T
  InfCtl --> Term
  Term --> Hold
  InfCtl --> Headless
  InfCtl -->|"-inferior-tty-set / --tty / dlv connect"| GC
  InfCtl --> DC
```

| Piece | Package / file | Holds |
|-------|----------------|-------|
| Debugger PTY | `ptyx.Client` | Master for GDB or Delve CLI |
| Inferior PTY (internal) | `ptyx.TTY` | Master for program stdio |
| GDB attach | `-inferior-tty-set /dev/pts/N` | Tells GDB which **slave** the program should use |
| Delve attach | `dlv exec --tty /dev/pts/N` | Same idea, only at **spawn** |
| `:b gdb` bridge | `cmd/gdbforge/gdb_console.go` | Subscribe/Send on debugger master |
| `:b io` bridge | `cmd/gdbforge/io_console.go` | Subscribe/Send on inferior master |
| External / headless | `cmd/gdbforge/inferior_tty.go` | Opens real terminals; rewires or restarts |

---

## Mode A — GDB with internal IO (default)

Two PTY pairs. gdbforge holds **both masters**.

```text
  User types in :b gdb                    User types in :b io
         │                                       │
         ▼                                       ▼
  ┌──────────────────┐                  ┌──────────────────┐
  │ ptyx.Client      │                  │ ptyx.TTY         │
  │ MASTER #1        │                  │ MASTER #2        │
  └────────┬─────────┘                  └────────┬─────────┘
           │                                     │
           ▼                                     ▼
  ┌──────────────────┐                  ┌──────────────────┐
  │ SLAVE #1         │                  │ SLAVE #2         │
  │ gdb --interpreter│                  │ /dev/pts/N       │
  │ =mi2             │── -inferior-tty-set ──►│  (path only) │
  └──────────────────┘                  └────────┬─────────┘
                                                 │
                                                 ▼
                                          under-debug app
                                          stdin/stdout/stderr
```

```mermaid
flowchart LR
  subgraph PTY1["PTY #1 · debugger"]
    M1["MASTER · ptyx.Client"]
    S1["SLAVE · gdb MI"]
    M1 <--> S1
  end

  subgraph PTY2["PTY #2 · inferior"]
    M2["MASTER · ptyx.TTY"]
    S2["SLAVE · /dev/pts/N"]
    M2 <--> S2
  end

  UI1[":b gdb"] <--> M1
  UI2[":b io"] <--> M2
  GDB["gdb"] --- S1
  GDB -.->|"-inferior-tty-set path"| S2
  Prog["inferior"] --- S2
```

### Who talks to whom

| From → To | Path |
|-----------|------|
| You → GDB | `:b gdb` → `Send` → **master #1** → slave → `gdb` |
| GDB → you | `gdb` → slave → **master #1** → Subscribe → GDB pane |
| You → program | `:b io` → `Send` → **master #2** → slave → program stdin |
| Program → you | program stdout → slave → **master #2** → Subscribe → IO pane |
| GDB → program tty | One MI command: `-inferior-tty-set <SlaveName()>` (not a continuous pipe through GDB) |

There is **no** MI command that feeds program stdin. Stdin is always via the inferior PTY master (`:b io`) or an external terminal’s master.

`ptyx.TTY` keeps the slave FD open so `/dev/pts/N` remains valid while gdbforge holds the master.

---

## Mode B — GDB with an external terminal

Same **debugger** PTY #1. Inferior stdio moves to a **real terminal emulator**. gdbforge does **not** hold that PTY’s master.

```text
  :b gdb  ←→  MASTER #1  ←→  gdb (unchanged)

  External terminal emulator
    └── MASTER (emulator owns keyboard/display)
          └── SLAVE /dev/pts/N  ←── held open by:  tty > file; sleep infinity
                                    ▲
                                    │  GDB: -inferior-tty-set /dev/pts/N  (live, no restart)
                                    │
                               under-debug app
```

```mermaid
flowchart TB
  GDBW[":b gdb"] <--> M1["ptyx.Client MASTER"]
  M1 <--> GDB["gdb SLAVE"]

  Term["mate-terminal / kitty / …"]
  Term --- Mext["MASTER · held by emulator"]
  Mext <--> Sext["SLAVE /dev/pts/N"]
  Hold["sleep infinity"] --- Sext
  GDB -.->|"-inferior-tty-set"| Sext
  Prog["inferior"] --- Sext

  IOW[":b io"] -.->|"note only · unwired"| X[ ]
```

### How the external pts is created

1. `OpenExternalTTY` / `:set inferior-tty` / Lua `open_external_tty`
2. Spawn `GDBFORGE_TERMINAL` (e.g. `mate-terminal`) running roughly: `tty > /tmp/…; exec sleep infinity`
3. Read `/dev/pts/N` from the temp file
4. GDB: live `-inferior-tty-set` pointing at that path; close internal `ptyx.TTY`
5. Unwire `:b io` (shows a note — type in the other window)

**Do not** keep an internal master subscribed **and** point `-inferior-tty-set` at an external slave. Closing the external window does not auto-rewire — use `:set inferior-tty internal`.

---

## Mode C — Delve with internal / external `--tty`

Delve’s CLI still rides **`ptyx.Client`** (master #1). The inferior is attached with a **spawn flag**, not a live MI switch:

```bash
dlv exec --tty /dev/pts/N -- ./prog [args…]
```

| | GDB | Delve |
|--|-----|-------|
| Attach inferior tty | `-inferior-tty-set` **anytime** | `--tty` **only when starting** `dlv exec` |
| Change mid-session | live MI | **restart** Delve with a new `--tty` (app layer) |
| Console | `ptyx.Client` → `(gdb)` / MI | `ptyx.Client` → `(dlv)` CLI |

```mermaid
flowchart LR
  GDBW[":b gdb"] <--> M1["ptyx.Client · dlv CLI"]
  M1 <--> DLV["dlv exec"]
  DLV -->|"--tty at spawn"| S2["SLAVE /dev/pts/N"]
  IOW[":b io or external"] <--> M2["master of that pts"]
  M2 <--> S2
  Prog["Go program"] --- S2
```

Internal default: gdbforge’s `ptyx.TTY` master + Delve `--tty` slave path.  
External: hold-open pts like Mode B, then **restart** `dlv exec --tty …`.

---

## Mode D — Delve headless + TCP (preferred Go TUI)

For Go TUIs, mid-session `--tty` restart is awkward. Preferred flow (`:lua dlv_ext_port` / `dlv_port`):

1. Open a **real terminal** running **headless** Delve:  
   `dlv exec --headless --listen=127.0.0.1:PORT -- ./prog …`  
   The program inherits **that terminal’s** stdio (emulator master ↔ pts slave).
2. Tear down the local `dlv exec` session.
3. Start a new local client: `dlv connect 127.0.0.1:PORT` on a fresh **`ptyx.Client`** (debugger console only).
4. Mark IO external — `:b io` is a note; type in the headless window.

```mermaid
sequenceDiagram
  participant UI as gdbforge UI
  participant Conn as ptyx.Client · dlv connect
  participant TCP as TCP :PORT
  participant HDLV as headless dlv in external terminal
  participant Prog as Go program

  UI->>HDLV: spawn_terminal / spawn_dlv_headless
  Note over HDLV,Prog: Program stdio = that terminal PTY
  UI->>Conn: dlv connect addr
  Conn->>TCP: CLI / API
  TCP->>HDLV: headless server
  HDLV->>Prog: debug control
  Note over UI,Prog: No local inferior PTY master in gdbforge
```

```text
  External terminal
    MASTER (emulator) ←→ SLAVE
         │
         ├── headless dlv  (listens on TCP)
         └── Go program stdio (same tty)

  gdbforge
    :b gdb  ←→  ptyx.Client MASTER  ←→  `dlv connect` SLAVE
                      │
                      └── TCP ──► headless dlv
```

So: **control plane = TCP** (plus a local PTY only for the connect CLI); **stdio plane = the external terminal’s PTY**.

---

## How `:b io` connects

| Fact | Detail |
|------|--------|
| Widget | `OutputWidget` — does **not** own `*ptyx.TTY` |
| Wiring | `cmd/gdbforge/io_console.go` — `wireInferiorIO` / `unwireInferiorIO` |
| Read path | Inferior master → Subscribe → coalesced `InferiorOutputMsg` → `AppendInferior` |
| Write path | Enter → `TTY.Send`; Ctrl-C → `^C` on **inferior** master (not the debugger PTY) |
| External / headless | Unwired; note text only |

It is a **line console** (ANSI, newlines) — not a full VT. Curses / alternate-screen apps need Mode B or D.

---

## How `:b gdb` connects

| Fact | Detail |
|------|--------|
| Widget | `GDBWidget` / console pane |
| Wiring | `cmd/gdbforge/gdb_console.go` |
| Backend | `gdb.GDBClient` or `dlv.Client` over the same `ptyx.Client` |
| Read path | Debugger master → parser (`GdbInputState` / `dlv.InputState`) → paint |
| Write path | Enter → `Send(cmd)`; Tab completion / queries also use this PTY (with write lock) |

Ctrl-C while the inferior is **running** typically interrupts via the **debugger** path (GDB/Delve signal / `^C` on the debugger PTY), which is separate from typing `^C` into `:b io`.

---

## Side-by-side summary

| Scenario | Debugger master | Inferior stdio master | How inferior attaches |
|----------|-----------------|----------------------|------------------------|
| GDB + `:b io` | `ptyx.Client` | `ptyx.TTY` in gdbforge | `-inferior-tty-set` |
| GDB + external tty | `ptyx.Client` | Terminal emulator | `-inferior-tty-set` (live) |
| Delve + `:b io` | `ptyx.Client` | `ptyx.TTY` in gdbforge | `dlv exec --tty` at spawn |
| Delve + external tty | `ptyx.Client` | Terminal emulator | restart with `--tty` |
| Delve + `dlv_ext_port` | `ptyx.Client` (`dlv connect`) | Terminal emulator (headless window) | inherit tty; control via **TCP** |

---

## `:b io` flood vs `:set inferior-tty`

Internal IO (`ptyx.TTY` → coalesce → UI `PostEvent` → `OutputWidget`) shares the **same event loop** that paints panes and handles keys. gdbforge applies backpressure and prioritizes Ctrl-C so a `printf` storm should not hard-freeze the app, but **display smoothness will not match mate-terminal/kitty**. That is a **known GUI limitation** of embedding a line console in the debugger TUI.

**Prefer `:set inferior-tty` when you need:**

- Smooth scrolling under high-rate stdout
- A real VT (curses / alternate screen / full-screen TUI)
- Program I/O isolated from debugger chrome redraw

Bare `:set inferior-tty` opens `GDBFORGE_TERMINAL` and points GDB at that pts (`-inferior-tty-set`, live). `:set inferior-tty internal` restores `:b io`. Details: [USER_GUIDE.md](USER_GUIDE.md#why-set-inferior-tty-external-stdio), [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md#external-terminal-stdio-tui-targets).

---

## Key source map

| Concern | Path |
|---------|------|
| Debugger PTY mux | `internal/ptyx/client.go` |
| Inferior PTY pair | `internal/ptyx/tty.go` |
| GDB + dual PTY startup | `internal/gdb/gdb_client.go` |
| Delve `--tty` / connect | `internal/dlv/client.go` |
| External tty / headless / restart | `cmd/gdbforge/inferior_tty.go` |
| IO bridge | `cmd/gdbforge/io_console.go` |
| GDB/Delve console bridge | `cmd/gdbforge/gdb_console.go` |
| Lua: open tty / spawn / dlv | `internal/luahost/user_scripts.go`, `lua/dlv_ext_port`, `lua/remotegdb` |

---

## Related docs

- [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) — MI, dual PTY startup details, MCP, Delve parsing
- [ARCHITECTURE.md](ARCHITECTURE.md) — app-wide MVC / subsystems
- [EXEC_SHELL.md](EXEC_SHELL.md) — `:!` also uses `ptyx.Client` (separate from debugger)
- [LUA_API.md](LUA_API.md) — `open_external_tty`, `spawn_terminal`, `spawn_dlv_headless`, `dlv_connect`
- [../lua/README.md](../lua/README.md) — `dlv_ext_port`, `terminal_debug`, `remotegdb` recipes

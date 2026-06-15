# Plugin Architecture

cgdb-go is designed for **extensibility**: custom debugger panes, scripted automation, and user-defined workflows. This document describes the planned plugin system centered on **Lua** integration.

**Status:** design phase — no Lua runtime is embedded yet.

**Companion docs:** [ARCHITECTURE.md](ARCHITECTURE.md) · [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) · [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md)

---

## Table of contents

- [Goals](#goals)
- [Why Lua](#why-lua)
- [Architecture overview](#architecture-overview)
- [Feature panes](#feature-panes)
- [Automation workflows](#automation-workflows)
- [Plugin API surface (planned)](#plugin-api-surface-planned)
- [Security model](#security-model)
- [Comparison to alternatives](#comparison-to-alternatives)
- [Implementation phases](#implementation-phases)

---

## Goals

| Goal | Description |
|------|-------------|
| **Custom panes** | Users add register views, trace buffers, hardware monitors |
| **Workflow automation** | Script repetitive debug sequences (attach, load, break, run) |
| **CI integration** | Headless scripts drive debugger backends without UI |
| **Low coupling** | Plugins never import Go UI packages directly |
| **Safe defaults** | Sandboxed Lua; explicit permission for dangerous ops |

---

## Why Lua

| Factor | Lua | Alternative (Python/WASM) |
|--------|-----|---------------------------|
| Embedding size | Small (`gopher-lua`, `yuin/gopher-lua`) | Python heavy; WASM complex |
| Debugger ecosystem | GDB/LLDB use Python — Lua fills **UI script** niche | Python overlaps with GDB scripts |
| Sandboxing | Well-understood coroutine model | Python sandbox harder |
| cgdb precedent | cgdb uses Tcl — Lua is lighter modern equivalent | Tcl declining in new projects |

**Design decision:** Lua scripts extend **cgdb-go UI and session orchestration**, not replace GDB's own Python scripting. Avoid duplicating GDB's introspection API — delegate to `core.Debugger` instead.

---

## Architecture overview

```mermaid
flowchart TB
    subgraph UI["termui"]
        WidgetHost["Widget host / PluginPane"]
        Layout["Layout / split tree"]
    end

    subgraph PluginRuntime["Plugin runtime · planned"]
        LuaVM["Lua VM"]
        Bindings["Go ↔ Lua bindings"]
        Registry["Plugin registry"]
    end

    subgraph Domain["core"]
        Events["Event bus"]
        Debugger["Debugger interface"]
        Session["Session config"]
    end

    Layout --> WidgetHost
    WidgetHost --> LuaVM
    LuaVM --> Bindings
    Bindings --> Events
    Bindings --> Debugger
    Bindings --> Session
    Registry --> LuaVM
```

Plugins load from:

| Location | Purpose |
|----------|---------|
| `~/.config/cgdb-go/plugins/` | User plugins |
| `./.cgdb-go/plugins/` | Project-local plugins |
| Built-in `embed` | Shipping default panes |

---

## Feature panes

A **feature pane** is a Workspace leaf widget provided by a plugin.

```mermaid
flowchart LR
    Plugin["Lua plugin"]
    PaneAPI["Pane API"]
    Widget["PluginWidget adapter"]
    Split["Split tree leaf"]

    Plugin --> PaneAPI --> Widget --> Split
```

### Pane lifecycle (planned)

1. Plugin registers pane metadata (title, init function).
2. User runs `:plugin load trace` or config auto-loads.
3. Runtime creates `PluginWidget` implementing `termui.Widget`.
4. Lua `on_draw(canvas)` / `on_key(event)` callbacks fire each frame/event.

### Example use cases

| Pane | Function |
|------|----------|
| RTOS task list | Parse GDB `info threads`, custom sort/filter |
| Peripheral register map | SVD-driven register tree (embedded) |
| Trace buffer | Collect `*stopped` events over time |
| CI status | Show test harness connection |

**Design decision:** plugins draw through the same **Canvas** abstraction as native widgets — they do not get raw screen access.

---

## Automation workflows

Headless and semi-headless workflows run Lua without full UI:

```mermaid
sequenceDiagram
    participant Script as Lua script
    participant Core as core.Session
    participant DBG as Debugger backend
    participant UI as termui (optional)

    Script->>Core: session.attach(config)
    Core->>DBG: connect
    Script->>DBG: send("break main")
    Script->>DBG: send("continue")
    DBG-->>Script: on_stopped callback
    Script->>UI: optional update pane
```

Use cases:

| Workflow | Mode |
|----------|------|
| Nightly regression | Headless — no `TermApp` |
| Repeatable bring-up | Script + visible UI |
| Custom `:command` | Lua registers command handler |

**Design decision:** automation APIs are a **subset** of pane APIs — same Lua module, different entry point (`main()` vs pane callbacks).

---

## Plugin API surface (planned)

### Events

```lua
-- Subscribe to domain events
on_event("BreakpointHit", function(ev)
  pane:append_line("hit at " .. ev.line)
end)
```

Maps to Go `core.Event` types.

### Debugger

```lua
debugger.send("info registers")
debugger.on_output(function(text) ... end)
```

Maps to `core.Debugger` + output channel.

### UI

```lua
pane:set_title("My Trace")
pane:draw_text(0, 0, "hello", "bold")
ui.command("my-cmd", function(args) ... end)
```

Maps to `Canvas` drawing helpers and command registry.

### Layout

```lua
ui.open_pane("my-plugin.trace", { direction = "vertical" })
```

Maps to `Layout.NewSplit` / focus APIs.

---

## Security model

| Level | Capability |
|-------|------------|
| **Trusted** (built-in) | Full API |
| **User plugin** | Debugger send, pane draw, file read in config dir |
| **Project plugin** | Requires user opt-in on first load |

Dangerous operations (`os.execute`, arbitrary file write) are **not exposed** in the default binding set. Network access requires explicit plugin manifest flag.

---

## Comparison to alternatives

| Approach | Pros | Cons |
|----------|------|------|
| **Lua embed** (chosen) | Small, fast, sandbox-friendly | New ecosystem for users |
| **GDB Python only** | Rich introspection | No UI integration |
| **External IPC** | Strong isolation | Latency, complexity |
| **WASM plugins** | Strong sandbox | Heavy toolchain |

---

## Implementation phases

| Phase | Deliverable |
|-------|-------------|
| 1 | `PluginWidget` stub + manual Go plugin registration |
| 2 | Embed `gopher-lua`; event + debugger bindings |
| 3 | Pane draw/key callbacks |
| 4 | `:plugin` commands + config discovery |
| 5 | Headless automation entry point |

Tracker: [ROADMAP.md](ROADMAP.md).

---

## Related documentation

- [DEBUGGER_INTEGRATION.md](DEBUGGER_INTEGRATION.md) — backend interfaces plugins call
- [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) — Widget / Canvas contract
- [INPUT.md](INPUT.md) — command registration

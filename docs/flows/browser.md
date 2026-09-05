---
title: Flow browser
description: Search curated gdbforge code paths — Tab completion, Ctrl-C, stop pipeline, and more. Separate from site-wide documentation search.
flow_browser: true
---

# Flow browser

Use the **search box below** to find execution paths (function call trees with file:line links).

This search is **only for code flows**, not the MkDocs header search (which indexes prose manuals like INPUT.md and ARCHITECTURE.md).

Pick a flow below, or search (e.g. `tab dlv`, `ctrl-c`, `stop`). The call tree expands inline
under the row you click; click the same row again to collapse it. Deep links: `#dlv-console-tab`,
`#stop-pipeline`, …

<div id="flow-browser" class="flow-browser">
  <div class="flow-browser__toolbar">
    <input
      id="flow-search"
      class="flow-browser__search md-input"
      type="search"
      placeholder="Search flows… (e.g. tab dlv break, ctrl-c, stop)"
      autocomplete="off"
      spellcheck="false"
    />
    <div class="flow-browser__filters" id="flow-filters" aria-label="Backend filter"></div>
  </div>
  <div id="flow-results" class="flow-browser__results" role="region" aria-label="Matching flows">
    <p class="flow-browser__empty">Loading flows…</p>
  </div>
  <div id="flow-detail" class="flow-browser__detail" role="region" aria-label="Selected flow" hidden></div>
</div>

## How the catalog is built

Call-graph analysis is done by the official Go tool
[`golang.org/x/tools/cmd/callgraph`](https://pkg.go.dev/golang.org/x/tools/cmd/callgraph),
pinned in `go.mod` via a `tool` directive and run as `go tool callgraph -algo vta`.
`cmd/flowdoc` parses its edge list and renders trees into [`flows.json`](flows.json).

| Layer | Source | Meaning |
|-------|--------|---------|
| **Curated** | `flows:` in [`flows.spec.yaml`](flows.spec.yaml) | Real user triggers (Tab, Ctrl-C, `:gdb continue`, …) with GDB/Delve branches |
| **Auto** | `discover:` in the same file | One tree per reachable function found by `callgraph`; marked **auto** |

Auto flows are **reachable static paths** from `main`, not recordings — an edge may not fire for a given user action.

```bash
go tool callgraph -algo vta ./cmd/gdbforge   # raw edges (the analyzer)
go run ./cmd/flowdoc --generate              # edges -> flows.json
go run ./cmd/flowdoc --check                 # validate file:line links
```

CI runs `--generate` before MkDocs on every docs deploy, so the catalog tracks the code.

See also [INPUT.md](../INPUT.md) and [DEBUGGER_INTEGRATION.md](../DEBUGGER_INTEGRATION.md) for prose documentation.

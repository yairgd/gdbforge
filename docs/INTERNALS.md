---
title: gdbforge Internals — Architecture, Code Flows and Contributing
description: Developer documentation for gdbforge — architecture, PTY and debugger integration, repository layout, code flows, and the termforge framework it is built on.
---

# Internals and contributing

How gdbforge is built and how to work on it. If you only want to debug something, start
from the [home page](README.md) and the [user guide](USER_GUIDE.md) instead.

## Developer documentation

The architecture — the MVC split, controllers and host interfaces, the event bus, and the
repository layout — is documented separately:

| Page | What is in it |
|------|---------------|
| [Overview](OVERVIEW.md) | Goals, motivation, and how gdbforge compares to cgdb and the GDB TUI |
| [Architecture](ARCHITECTURE.md) | Subsystems and data flow. Start with [MVC](ARCHITECTURE.md#mvc-current), [built on termforge](ARCHITECTURE.md#built-on-termforge), and the [design principles](ARCHITECTURE.md#design-principles) |
| [PTY architecture](PTY_ARCHITECTURE.md) | Dual PTY master/slave, GDB versus Delve, `:b io`, external terminal |
| [Debugger integration](DEBUGGER_INTEGRATION.md) | GDB MI2, the unified `backend.Backend`, the [Delve backend](DEBUGGER_INTEGRATION.md#delve-backend-peer-of-gdb), `:AI` / GdbMcpService |
| [Window management](WINDOW_MANAGEMENT.md) | The three-band root layout, split trees, tabs, command line |
| [Directory structure](DIRECTORY_STRUCTURE.md) | Repository layout and the responsibility of each package |
| [Dependencies](DEPENDENCIES.md) | Go modules and the import rules between the debugger and the framework |
| [Developer guide](DEVELOPER_GUIDE.md) | Onboarding, which files to read in what order, common pitfalls |
| [Flow browser](flows/browser.md) | Curated call trees (Tab completion, Ctrl-C, the stop pipeline) with links to source. Has its own search box, separate from this site's header search |
| [Roadmap](ROADMAP.md) · [Releasing](RELEASING.md) · [Hosting](HOSTING.md) | Planned work, how releases are cut, how these docs are built |

The generic terminal UI machinery — the widget system, the rendering pipeline, and the
split-tree engine — lives in **termforge** and is documented on its own site:
[UI architecture](https://yairgd.github.io/termforge/UI_ARCHITECTURE/) ·
[rendering](https://yairgd.github.io/termforge/RENDERING/) ·
[window management](https://yairgd.github.io/termforge/WINDOW_MANAGEMENT/).

Mermaid diagram sources live under
[`docs/diagrams/`](https://github.com/yairgd/gdbforge/tree/main/docs/diagrams).

### Reading these docs locally

```bash
python3 -m pip install -r requirements-docs.txt
./docs/serve.sh          # or: task docs
```

Then open <http://127.0.0.1:8765/>. Details: [Hosting](HOSTING.md).

---

## Related project: termforge

gdbforge is the debugger. The terminal UI it runs on is a separate project,
**[termforge](https://yairgd.github.io/termforge/)** — widgets, split-tree windows, tabs,
colon commands with tab completion, key-sequence bindings, and the terminal emulator pane,
with **no debugger in it**. termforge was extracted *from* gdbforge once that machinery
stood on its own, so gdbforge is both its origin and its largest consumer. This repository
is now the debugger only.

| Question | Site |
|----------|------|
| How do I debug something with GDB, Delve, an embedded target, or kgdb? | **This site** |
| How does a widget, split tree, or `:command` work in general? | [termforge documentation](https://yairgd.github.io/termforge/) — [UI architecture](https://yairgd.github.io/termforge/UI_ARCHITECTURE/), [window management](https://yairgd.github.io/termforge/WINDOW_MANAGEMENT/), [rendering](https://yairgd.github.io/termforge/RENDERING/) |
| How do I build my own terminal app on the same framework? | [termforge documentation](https://yairgd.github.io/termforge/) |
| How does gdbforge drive GDB or Delve? | [Debugger integration](DEBUGGER_INTEGRATION.md) on this site |

Source: [github.com/yairgd/termforge](https://github.com/yairgd/termforge) ·
how the split works: [ARCHITECTURE.md — Built on termforge](ARCHITECTURE.md#built-on-termforge).

Contribution workflow: [CONTRIBUTING.md](https://github.com/yairgd/gdbforge/blob/main/CONTRIBUTING.md) ·
[issue tracker](https://github.com/yairgd/gdbforge/issues).

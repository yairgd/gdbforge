# cgdb-go

**cgdb-go** is a terminal debugger UI in Go, inspired by [cgdb](https://github.com/cgdb/cgdb). It provides a composable split-pane workspace, a `termui` framework (tcell), GDB MI2 integration, and a Vim-style `:` command line.

```bash
go run ./cmd/cgdb
```

Module: `github.com/yairgd/cgdb-go`

Documentation: [docs/README.md](docs/README.md) · local server: `./docs/serve.sh`

---

## Repository layout

```text
cgdb-go/
├── cmd/cgdb/           # Debugger application entry point
├── cmd/docserve/       # Documentation server
├── internal/termui/    # TUI framework (standalone-ready)
├── internal/cgdb/      # App-specific debugger widgets
├── internal/core/      # Domain: buffers, debugger interface
├── internal/gdb/       # GDB MI2 backend
└── docs/               # Architecture and developer guides
```

Legacy Bubble Tea chat code lives under `cmd/tui` and `internal/ui/tui` — separate from cgdb-go.

---

# Legacy: PromptCore chatbot

PromptCore is a lightweight AI context engine (chat TUI). See below for the original chat project structure.

It is built with a clean architecture that separates:

- Core logic (context + AI orchestration)
- Application layer (events + state)
- UI adapters (TUI, future Web UI, etc.)

The goal is to provide a reusable AI context infrastructure that can power:

- Terminal interfaces (TUI)
- Web interfaces
- API servers
- Debugging assistants
- Developer tools

---

## Vision

PromptCore is not just a chat application.

It is a context engine.

It manages:
- Conversation history
- AI orchestration
- Command handling
- Future RAG integration
- Pluggable UI adapters

---

# Project Structure

```bash
cgdb-go/
├── go.mod
├── go.sum
│
├── cmd/
│   ├── cgdb/              # cgdb-go debugger
│   ├── docserve/          # docs server
│   └── tui/               # legacy chat TUI
│
├── internal/
│   ├── termui/            # TUI framework
│   ├── cgdb/widgets/      # debugger panes
│   ├── core/
│   ├── gdb/
│   └── ui/tui/            # legacy Bubble Tea
│
└── docs/
```

---

# Debugging with Delve (dlv)

The debugger app can be debugged using Delve in headless mode.

## 1. Start Delve in Headless Mode

Open a terminal in the project root directory and run:

```bash
dlv debug ./cmd/cgdb --headless --listen=:2346 --api-version=2
```

This will compile the project, start the debugger, listen on port 2346, and wait for a client.

## 2. Connect to the Debugger

Open a second terminal and run:

```bash
dlv connect :2346
```

## 3. Useful Delve Commands

Inside the Delve prompt you can use:

```
b main.main        # set breakpoint
c                  # continue execution
n                  # next line
s                  # step into
bt                 # backtrace
p variableName     # print variable
q                  # quit debugger
```

## 4. Notes

- Make sure the port number matches in both commands.
- Always run the headless debugger from the project root (where `go.mod` is located).
- To debug another command under `cmd/`, change the path accordingly (e.g. `./cmd/tui` for the legacy chat TUI).


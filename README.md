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
├── internal/core/      # Domain: buffers, debugger interface, events
├── internal/gdb/       # GDB MI2 backend
└── docs/               # Architecture and developer guides
```

---

## Debugging with Delve (dlv)

The debugger app can be debugged using Delve in headless mode.

### 1. Start Delve in headless mode

From the project root:

```bash
dlv debug ./cmd/cgdb --headless --listen=:2346 --api-version=2
```

### 2. Connect to the debugger

In a second terminal:

```bash
dlv connect :2346
```

### 3. Useful Delve commands

```
b main.main        # set breakpoint
c                  # continue execution
n                  # next line
s                  # step into
bt                 # backtrace
p variableName     # print variable
q                  # quit debugger
```

Always run Delve from the project root (where `go.mod` lives).

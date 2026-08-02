# Contributing to gdbforge

Thank you for your interest in contributing.

## Philosophy

- Keep the core clean and UI-agnostic.
- Avoid premature abstraction.
- Prefer clarity over cleverness.
- Write granular commits.

---

## Workflow

1. Create a feature branch:

   git checkout -b feature/your-feature-name

2. Make small, logical commits.

3. Rebase onto main before submitting:

   git rebase main

4. Open a Pull Request.

---

## Commit Message Convention

Use clear prefixes:

- feat: new feature
- fix: bug fix
- refactor: internal restructuring
- docs: documentation update
- chore: maintenance task

Example:

feat: add command execution engine

---

## Code Guidelines

- Keep packages focused.
- Do not mix UI logic inside core.
- Avoid global state.
- Keep functions small and testable.
- Write comments in English.

---

## Testing

```bash
go test ./...                 # always works (no extra tools)
task test                     # same via Taskfile.yml — needs go-task on PATH
```

If `task: command not found`, either use `go test` above, or install [go-task](https://taskfile.dev) and put `$(go env GOPATH)/bin` on `PATH`:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
export PATH="$PATH:$(go env GOPATH)/bin"   # add to shell rc if needed
```

Tests live next to the code as `*_test.go` (same package). Most are pure unit tests.

One package:

```bash
go test ./internal/luahost/
```

One test (`-run` is a regex; Go tests a package, not a single `*_test.go` file):

```bash
go test ./internal/luahost/ -run TestDispatchTick
```

A few packages exercise real GDB, Delve, or PTYs and **skip** when those tools (or the repo-root `hello` binary) are missing. That is expected; a green `go test ./...` without gdb/dlv still means unit coverage passed.

Before a PR, also useful:

```bash
go vet ./...
./scripts/check_imports.sh
# or, with go-task on PATH: task vet && task check-imports
```

---

## Questions

Open an issue if you are unsure before implementing large changes.

# Contributing to cgdb-go

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

Run tests:

go test ./...

---

## Questions

Open an issue if you are unsure before implementing large changes.

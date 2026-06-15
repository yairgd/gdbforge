# Rendering System

cgdb-go renders through an off-screen **Grid** of **Cells**, composed by widgets via **Canvas**, and flushed to **tcell**. This document covers the cell model, border drawing, Unicode text, screen synchronization, and the path to **diff-based rendering**.

**Companion docs:** [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) · [ARCHITECTURE.md](ARCHITECTURE.md)

---

## Table of contents

- [Rendering overview](#rendering-overview)
- [Grid](#grid)
- [Cell model](#cell-model)
- [Border composition](#border-composition)
- [Unicode and text drawing](#unicode-and-text-drawing)
- [Screen synchronization](#screen-synchronization)
- [Future diff rendering](#future-diff-rendering)
- [Known gaps](#known-gaps)

---

## Rendering overview

```text
Widgets
    ↓
Canvas        (local Rect, optional Grid for borders)
    ↓
Grid          ([][]Cell framebuffer)
    ↓
tcell Screen  (terminal backend)
```

```mermaid
flowchart LR
    W["Widgets"]
    C["Canvas<br/>(local Rect)"]
    G["Grid<br/>(Cell framebuffer)"]
    T["tcell Screen"]

    W -->|"Draw(c Canvas)"| C
    C -->|"writes Cells"| G
    G -->|"Draw / diff"| T
```

*Source: [`diagrams/rendering_pipeline.mermaid`](diagrams/rendering_pipeline.mermaid)*

**Design rationale:** an intermediate grid decouples **what changed** from **how the terminal is updated**. Without it, every widget would call `screen.SetContent` directly, making dirty tracking impossible.

---

## Grid

```go
type Grid struct {
    W, H int
    Cells [][]Cell
}
```

| Method | Purpose |
|--------|---------|
| `NewGrid(w, h)` | Allocate cell storage |
| `DrawVertical(x, y1, y2, bold)` | Mark vertical edge segments |
| `DrawHorizontal(y, x1, x2, bold)` | Mark horizontal edge segments |
| `Draw(screen, style)` | Compose runes and flush non-space cells to tcell |
| `Clear(screen, style)` | Reset all cells to space |

`TermApp` holds:

| Buffer | Role |
|--------|------|
| `backBuffer` | Draw target (resize allocates both) |
| `frontBuffer` | Displayed frame; currently the flush source |

On terminal resize, `UpdateCanvas()` calls `screen.Sync()`, reads new dimensions, and reallocates both grids.

Implementation: `grid.go`, `term_app.go`.

---

## Cell model

```go
type Cell struct {
    Up, Down, Left, Right bool  // edge flags
    Rune                  rune  // composed output
    Bold                  bool  // heavy vs light box drawing
}
```

Cells are **edge-centric**, not character-centric, during layout:

1. Border drawing sets edge flags on grid cells.
2. Before flush, `EdgesToRune()` composes a Unicode box-drawing rune from the flags.
3. `Grid.Draw` writes composed runes to tcell.

**Design decision:** edge flags allow adjacent panes to **share** border cells without double-drawing. When a vertical split meets a horizontal split, corner cells resolve to crosses or tees automatically.

### Edge-to-rune mapping

`Cell.EdgesToRune()` handles:

| Pattern | Light | Bold |
|---------|-------|------|
| Cross (+) | `┼` | `╋` |
| T-junctions | `├┤┬┴` | `┣┫┳┻` |
| Corners | `┌┐└┘` | `┏┓┗┛` |
| Vertical line | `│` | `┃` |
| Horizontal line | `─` | `━` |
| No edges | space | space |

Comment in source notes that **mixed-weight corners** (light meeting bold) are not yet handled — a future enhancement when focus highlighting uses bold borders.

Implementation: `cell.go`.

---

## Border composition

Split layout draws borders during `BuildLayout`:

```go
// Vertical split at column leftW
c.DrawVerticalLocal(leftW, 0, c.H(), false)

// Horizontal split at row topH
c.DrawHorizontalLocal(topH, 0, c.W(), false)
```

These call into `Grid.DrawVertical` / `DrawHorizontal`, setting edge flags on the separator line.

```mermaid
flowchart TB
    Layout["BuildLayout"]
    DV["DrawVerticalLocal"]
    DH["DrawHorizontalLocal"]
    Grid["Grid edge flags"]
    Compose["EdgesToRune"]
    Tcell["screen.SetContent"]

    Layout --> DV --> Grid
    Layout --> DH --> Grid
    Grid --> Compose --> Tcell
```

**Design decision:** borders belong to the **layout engine**, not widgets. Widgets should not draw their own outer frame — this prevents double borders and misaligned corners in nested splits.

**Planned:** focused pane gets `bold=true` on its bordering edges for visual feedback.

---

## Unicode and text drawing

### UTF-8 text

`Canvas.DrawANSIText` iterates UTF-8 runes and calls `SetContent` per column:

```go
func (c Canvas) DrawANSIText(localX, localY int, text string, baseStyle tcell.Style)
```

- Uses `utf8.DecodeRuneInString` for correct wide-character iteration.
- Clips at canvas width.
- ANSI escape parsing exists but is **disabled** (`if false` block) — reserved for future styled GDB output.

**Gap:** no grapheme cluster / East Asian width handling yet. For debugger source code (mostly ASCII), this is acceptable short-term. Source view will need `runewidth` or equivalent before internationalized code display.

### Box drawing

Border runes use Unicode **Box Drawing** block (U+2500–U+257F) with **Heavy** variants (U+2501+) for bold edges.

Terminal emulators with UTF-8 enabled (the default in modern terminals) render these correctly. Fallback to ASCII `+--|` is not implemented — a future compatibility mode.

Implementation: `utf.go`, `cell.go`.

---

## Screen synchronization

Current frame loop (`TermApp.Run`):

```go
for !app.exit {
    ev := app.screen.PollEvent()
    app.HandleEvent(ev)
    for _, w := range app.widgets {
        w.HandleEvent(ev)
    }
    app.Draw(Canvas{app.screen, app.canvas.Rect(), app.frontBuffer})
    app.frontBuffer.Draw(app.screen, tcell.StyleDefault)
    app.screen.Show()
}
```

| Step | Purpose |
|------|---------|
| `PollEvent` | Block for input or resize |
| `HandleEvent` | Global keys, resize → `UpdateCanvas` |
| `Draw` | Widgets render |
| `frontBuffer.Draw` | Grid cells → tcell |
| `Show` | Batch update to terminal |

On resize (`EventResize`):

1. `screen.Sync()` reconciles internal size state.
2. New `Grid` allocated at updated dimensions.
3. Widgets receive resize events on next poll.

**Design decision:** single-threaded draw loop — no concurrent `SetContent` calls. GDB output uses `PostEvent` to marshal async data onto this thread.

---

## Future diff rendering

**Goal:** send only **modified cells** to tcell each frame, reducing bandwidth for remote sessions and large terminals.

Planned algorithm:

```mermaid
flowchart LR
    Draw["Widget Draw"]
    Back["backBuffer"]
    Front["frontBuffer"]
    Diff["Compare cells"]
    Patch["SetContent changed only"]
    Swap["Swap buffers"]

    Draw --> Back
    Back --> Diff
    Front --> Diff
    Diff --> Patch
    Patch --> Swap
```

Steps:

1. Clear `backBuffer`.
2. Widgets draw into `backBuffer` (all `SetContent` routes through grid — prerequisite).
3. Compare `backBuffer.Cells` vs `frontBuffer.Cells` (rune + style).
4. Emit `screen.SetContent` only for diffs.
5. Swap buffer pointers.

Additional optimizations:

| Technique | Benefit |
|-----------|---------|
| Per-widget dirty flags | Skip draw for unchanged panes |
| Damage regions | Limit diff to affected rects |
| Idle skip | No flush when no events and no dirty |

**Prerequisite work:** route widget text through `Grid` instead of direct tcell writes. See [Known gaps](#known-gaps).

---

## Known gaps

| Gap | Impact | Mitigation plan |
|-----|--------|-----------------|
| Widget text bypasses Grid | Diff rendering blocked | Add `Canvas.SetCell` writing to grid |
| Full grid flush every frame | Wasted I/O on large terminals | Implement diff pass |
| No style in Cell | Diff cannot track color changes | Extend Cell with style ID |
| ANSI parsing disabled | GDB color output ignored | Enable ANSI path in DrawANSIText |
| No wide-char width | Misaligned columns for CJK | Integrate runewidth |
| Mixed bold/light corners | Visual glitches at focus borders | Corner weight resolver |

---

## Related documentation

- [UI_ARCHITECTURE.md](UI_ARCHITECTURE.md) — Canvas API
- [WINDOW_MANAGEMENT.md](WINDOW_MANAGEMENT.md) — split separators
- [ROADMAP.md](ROADMAP.md) — diff rendering milestone

package widgets

import (
	"sync"
	"time"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/platform"
	"github.com/yairgd/gdbforge/internal/termui"
)

// cell is one painted glyph in the Lua pane grid.
type luaCell struct {
	ch    rune
	color string
}

// LuaWidget hosts a Lua script with cell draw, keys, tick, and gdbforge.print.
type LuaWidget struct {
	termui.BaseWidget
	rt *luahost.Runtime

	mu       sync.Mutex
	cells    [][]luaCell
	gridW    int
	gridH    int
	logLines []string // gdbforge.print scrollback (drawn below / overlay top)

	tickEvery    time.Duration
	stopTick     chan struct{}
	requestFrame func()

	lastTick time.Time
	useTick  bool
}

// NewLuaWidget loads src into a dedicated Lua VM bound to this pane.
func NewLuaWidget(title, src string, onRegister luahost.OnRegister) (*LuaWidget, error) {
	w := &LuaWidget{
		BaseWidget: termui.BaseWidget{PaneName: title},
		tickEvery:  100 * time.Millisecond,
		logLines:   nil,
	}
	w.rt = luahost.New(w, onRegister)
	if err := w.rt.LoadString(src, title); err != nil {
		w.rt.Close()
		return nil, err
	}
	w.useTick = true
	w.lastTick = time.Now()
	return w, nil
}

// Runtime returns the owned Lua host.
func (w *LuaWidget) Runtime() *luahost.Runtime { return w.rt }

// Close tears down the Lua VM and tick goroutine.
func (w *LuaWidget) Close() {
	w.StopTicks()
	if w.rt != nil {
		w.rt.Close()
		w.rt = nil
	}
}

// SetFrameRequester wires RequestFrame for the game tick loop.
func (w *LuaWidget) SetFrameRequester(fn func()) {
	w.requestFrame = fn
}

// StartTicks begins posting frames while the pane is active (ModeLua).
func (w *LuaWidget) StartTicks() {
	w.StopTicks()
	if !w.useTick || w.requestFrame == nil {
		return
	}
	stop := make(chan struct{})
	w.stopTick = stop
	go func() {
		t := time.NewTicker(w.tickEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if w.requestFrame != nil {
					w.requestFrame()
				}
			}
		}
	}()
}

// StopTicks stops the background ticker.
func (w *LuaWidget) StopTicks() {
	if w.stopTick != nil {
		close(w.stopTick)
		w.stopTick = nil
	}
}

// AppendPrint implements luahost.Pane (gdbforge.print).
func (w *LuaWidget) AppendPrint(s string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.logLines = append(w.logLines, s)
	const maxLog = 200
	if len(w.logLines) > maxLog {
		w.logLines = w.logLines[len(w.logLines)-maxLog:]
	}
}

// ClearAll implements luahost.Pane (gdbforge.clear).
func (w *LuaWidget) ClearAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.logLines = nil
	w.clearCellsLocked()
}

// ClearCells implements luahost.Pane (pane.clear).
func (w *LuaWidget) ClearCells() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.clearCellsLocked()
}

func (w *LuaWidget) clearCellsLocked() {
	for y := range w.cells {
		for x := range w.cells[y] {
			w.cells[y][x] = luaCell{ch: ' '}
		}
	}
}

// SetCell implements luahost.Pane (pane.set_cell).
func (w *LuaWidget) SetCell(x, y int, ch rune, color string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if y < 0 || x < 0 || y >= w.gridH || x >= w.gridW {
		return
	}
	if ch == 0 {
		ch = ' '
	}
	w.cells[y][x] = luaCell{ch: ch, color: color}
}

// Size implements luahost.Pane (pane.size).
func (w *LuaWidget) Size() (int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.gridW, w.gridH
}

// Clear implements termui.Clearable (:clear).
func (w *LuaWidget) Clear() {
	w.ClearAll()
}

// HandleLuaKey delivers a key to on_key (ModeLua). Returns true if consumed.
func (w *LuaWidget) HandleLuaKey(ev *tcell.EventKey) bool {
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		return true
	}
	if w.rt != nil {
		w.rt.DispatchKey(key.String())
	}
	return true
}

// HandleEvent handles mouse (ignored) and keys when routed via the tab.
func (w *LuaWidget) HandleEvent(ev tcell.Event) {
	if e, ok := ev.(*tcell.EventKey); ok {
		w.HandleLuaKey(e)
	}
}

// Draw runs on_tick/on_draw then blits the cell grid (+ print log at bottom).
func (w *LuaWidget) Draw(c termui.Canvas) {
	w.ensureGrid(c.W(), c.H())
	now := time.Now()
	dt := now.Sub(w.lastTick).Seconds()
	if dt < 0 {
		dt = 0
	}
	w.lastTick = now
	if w.rt != nil {
		if w.useTick {
			w.rt.DispatchTick(dt)
		}
		w.rt.DispatchDraw()
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	def := tcell.StyleDefault
	for y := 0; y < w.gridH && y < c.H(); y++ {
		for x := 0; x < w.gridW && x < c.W(); x++ {
			cell := w.cells[y][x]
			ch := cell.ch
			if ch == 0 {
				ch = ' '
			}
			c.SetContent(x, y, ch, styleForColor(cell.color, def))
		}
	}
	// Overlay last print lines at the bottom if any.
	if n := len(w.logLines); n > 0 && c.H() > 0 {
		maxShow := c.H()
		if maxShow > c.H() - 1 {
			maxShow = c.H()-1
		}
		start := n - maxShow
		if start < 0 {
			start = 0
		}
		row := c.H() - (n - start)
		if row < 0 {
			row = 0
		}
		st := def.Foreground(tcell.ColorGray)
		for i := start; i < n && row < c.H(); i++ {
			line := w.logLines[i]
			x := 0
			for _, r := range line {
				if x >= c.W() {
					break
				}
				c.SetContent(x, row, r, st)
				x++
			}
			row++
		}
	}
}

func (w *LuaWidget) ensureGrid(width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	if width == w.gridW && height == w.gridH && w.cells != nil {
		return
	}
	w.gridW = width
	w.gridH = height
	w.cells = make([][]luaCell, height)
	for y := 0; y < height; y++ {
		w.cells[y] = make([]luaCell, width)
		for x := 0; x < width; x++ {
			w.cells[y][x] = luaCell{ch: ' '}
		}
	}
}

func styleForColor(name string, base tcell.Style) tcell.Style {
	if name == "" {
		return base
	}
	if c, ok := platform.ParseColorName(name); ok {
		return base.Foreground(c)
	}
	switch name {
	case "green":
		return base.Foreground(tcell.ColorGreen)
	case "red":
		return base.Foreground(tcell.ColorRed)
	case "yellow":
		return base.Foreground(tcell.ColorYellow)
	case "cyan":
		return base.Foreground(tcell.ColorAqua)
	case "magenta", "purple":
		return base.Foreground(tcell.ColorPurple)
	case "blue":
		return base.Foreground(tcell.ColorBlue)
	case "white":
		return base.Foreground(tcell.ColorWhite)
	default:
		return base
	}
}

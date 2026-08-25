package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/commands"
	"github.com/yairgd/gdbforge/internal/platform"
)

type BaseWidget struct {
	Ctx      platform.AppContext
	PaneName string

	// Per-widget key chord trie; only consulted for the focused insert-mode pane.
	keys *commands.KeyBindingRegistry

	// focused is set by the layout before Draw when this leaf has focus.
	focused bool
	// cursor paints the caret for widgets that type/navigate (default: system block).
	cursor CellCursor
}

func NewBaseWidget(ctx platform.AppContext) BaseWidget {
	return BaseWidget{
		Ctx:    ctx,
		keys:   commands.NewKeyBindingRegistry(),
		cursor: NewNativeCursor(),
	}
}

// SetFocused is called by WidgetTree before Draw.
func (b *BaseWidget) SetFocused(focused bool) {
	b.focused = focused
}

func (b *BaseWidget) Focused() bool {
	return b.focused
}

// SetCursor replaces the widget caret (NativeCursor / InverseCursor / custom).
func (b *BaseWidget) SetCursor(c CellCursor) {
	if c == nil {
		c = NewNativeCursor()
	}
	b.cursor = c
}

func (b *BaseWidget) Cursor() CellCursor {
	if b.cursor == nil {
		b.cursor = NewNativeCursor()
	}
	return b.cursor
}

// PaintCursor draws the caret when this widget is focused.
func (b *BaseWidget) PaintCursor(c Canvas, x, y int, under rune) {
	if !b.focused {
		return
	}
	b.Cursor().Paint(c, x, y, under)
}

func (b *BaseWidget) ensureKeys() {
	if b.keys == nil {
		b.keys = commands.NewKeyBindingRegistry()
	}
}

// BindKey registers shortcut chords for this widget (same syntax as app keybindings).
func (b *BaseWidget) BindKey(cmd *commands.CommandNode, bindings ...string) {
	b.ensureKeys()
	b.keys.Bind(cmd, bindings...)
}

// BindKeyFunc is a convenience wrapper around BindKey.
func (b *BaseWidget) BindKeyFunc(name string, action func(args ...any), bindings ...string) {
	b.BindKey(commands.NewCommand(name, action), bindings...)
}

// HandleBoundKey tries the widget's key trie. Returns true if the key was
// consumed (completed binding or unfinished chord). Handled bindings that
// return false are not consumed (fall through).
func (b *BaseWidget) HandleBoundKey(ev *tcell.EventKey) bool {
	if b.keys == nil {
		return false
	}
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		b.keys.ResetPartial()
		return false
	}
	completed, handled := b.keys.HandleKey(key)
	if !handled {
		return false
	}
	if !completed {
		return b.keys.InPartial()
	}
	return true
}

func (b *BaseWidget) ResetKeyPartial() {
	if b.keys != nil {
		b.keys.ResetPartial()
	}
}

// StatusLabel is the copyable status-band text (default: PaneName).
func (b *BaseWidget) StatusLabel() string {
	if b == nil {
		return ""
	}
	return b.PaneName
}

func (b *BaseWidget) DrawStatusLine(c Canvas, active bool) {
	name := b.StatusLabel()
	if name == "" {
		return
	}
	if b.focused {
		PaintStatusBar(c, name, active)
		return
	}
	PaintInactiveStatusBar(c, name)
}

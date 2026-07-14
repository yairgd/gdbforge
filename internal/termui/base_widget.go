package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/cgdb-go/internal/commands"
	"github.com/yairgd/cgdb-go/internal/platform"
)

const (
	CmdPushData CommandID = iota + 10
)

type BaseWidget struct {
	Events   chan Event // outbound -> app
	Inbox    chan Event // inbound  <- app
	Ctx      platform.AppContext
	PaneName string

	// Per-widget key chord trie; only consulted for the focused insert-mode pane.
	keys *commands.KeyBindingRegistry
}

func NewBaseWidget(ctx platform.AppContext) BaseWidget {
	return BaseWidget{
		Events: make(chan Event, 16),
		Inbox:  make(chan Event, 16),
		Ctx:    ctx,
		keys:   commands.NewKeyBindingRegistry(),
	}
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
// consumed (completed binding or unfinished chord).
func (b *BaseWidget) HandleBoundKey(ev *tcell.EventKey) bool {
	if b.keys == nil {
		return false
	}
	key, ok := platform.KeyFromEvent(ev)
	if !ok {
		b.keys.ResetPartial()
		return false
	}
	cmd, handled := b.keys.HandleKey(key)
	if !handled {
		return false
	}
	if cmd != nil && cmd.Action != nil {
		cmd.Action()
	}
	return true
}

func (b *BaseWidget) ResetKeyPartial() {
	if b.keys != nil {
		b.keys.ResetPartial()
	}
}

func (b *BaseWidget) Emit(ev Event) {
	if b.Events != nil {
		b.Events <- ev
	}
}

func (b *BaseWidget) Start(handler func(Event)) {
	go func() {
		for ev := range b.Inbox {
			handler(ev)
		}
	}()
}

func (b *BaseWidget) DrawStatusLine(c Canvas, active bool) {
	if b.PaneName != "" {
		PaintStatusBar(c, b.PaneName, active)
	}
}

// send event into the widget
func (b *BaseWidget) Post(cmd Command, args ...string) {
	if b.Events != nil {
		b.Events <- BaseEvent{
			Cmd:  cmd,
			Args: args,
		}
	}
}

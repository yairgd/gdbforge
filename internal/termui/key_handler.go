package termui

import (
	tcell "github.com/gdamore/tcell/v2"
	"github.com/yairgd/gdbforge/internal/platform"
)

type KeyHandler func(ev *tcell.EventKey) bool

type ModeKeyHandlers map[platform.Mode]KeyHandler

package platform

import (
	"sync"

	"golang.design/x/clipboard"
)

var (
	clipOnce sync.Once
	clipOK   bool
)

func initClipboard() {
	clipOK = clipboard.Init() == nil
}

// SetClipboardText copies text to the system clipboard via X11/Wayland (Linux),
// Win32, or Cocoa — no external tools such as xclip are required.
func SetClipboardText(text string) bool {
	if text == "" {
		return false
	}

	clipOnce.Do(initClipboard)
	if !clipOK {
		return false
	}

	clipboard.Write(clipboard.FmtText, []byte(text))
	return true
}

package platform

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"

	"golang.design/x/clipboard"
)

var (
	clipOnce sync.Once
	clipOK   bool

	extClipOnce sync.Once
	xclipPath   string
	xselPath    string
	wlCopyPath  string
)

func initClipboard() {
	clipOK = clipboard.Init() == nil
}

func initExtClip() {
	if p, err := exec.LookPath("xclip"); err == nil {
		xclipPath = p
	}
	if p, err := exec.LookPath("xsel"); err == nil {
		xselPath = p
	}
	if p, err := exec.LookPath("wl-copy"); err == nil {
		wlCopyPath = p
	}
}

// SetClipboardText copies text to the system CLIPBOARD (Ctrl+C / Ctrl+V)
// via X11/Wayland (Linux), Win32, or Cocoa.
func SetClipboardText(text string) bool {
	if text == "" {
		return false
	}

	clipOnce.Do(initClipboard)
	ok := false
	if clipOK {
		if _, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text)); err == nil {
			ok = true
		}
	}
	// Also push CLIPBOARD via xclip when available (helps some terminals).
	if writeXSelection("clipboard", text) {
		ok = true
	}
	return ok
}

// SetPrimaryText copies text to the X11/Wayland PRIMARY selection (middle-click
// paste in other apps). gdbforge stays running and serves paste requests via
// golang.design/x/clipboard; external tools are fallbacks when Init fails.
func SetPrimaryText(text string) bool {
	if text == "" {
		return false
	}

	clipOnce.Do(initClipboard)
	ok := false
	if clipOK {
		_, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text), clipboard.FromPrimary())
		if err == nil {
			ok = true
		}
	}
	if writePrimaryExternal(text) {
		ok = true
	}
	return ok
}

// GetClipboardText reads the system CLIPBOARD text.
func GetClipboardText() (string, bool) {
	clipOnce.Do(initClipboard)
	if clipOK {
		b, err := clipboard.Read(context.Background(), clipboard.FmtText)
		if err == nil && len(b) > 0 {
			return string(b), true
		}
		if err != nil && !errors.Is(err, clipboard.ErrNoData) {
			return readXSelection("clipboard")
		}
	}
	return readXSelection("clipboard")
}

// GetPrimaryText reads the X11/Wayland PRIMARY selection (middle-click source).
func GetPrimaryText() (string, bool) {
	clipOnce.Do(initClipboard)
	if clipOK {
		b, err := clipboard.Read(context.Background(), clipboard.FmtText, clipboard.FromPrimary())
		if err == nil && len(b) > 0 {
			return string(b), true
		}
	}
	return readXSelection("primary")
}

func writePrimaryExternal(text string) bool {
	extClipOnce.Do(initExtClip)
	if wlCopyPath != "" {
		cmd := exec.Command(wlCopyPath, "--primary")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	if xselPath != "" {
		cmd := exec.Command(xselPath, "--primary", "--input")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return writeXSelection("primary", text)
}

func writeXSelection(selection, text string) bool {
	extClipOnce.Do(initExtClip)
	if xclipPath == "" {
		return false
	}
	cmd := exec.Command(xclipPath, "-selection", selection, "-in")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

func readXSelection(selection string) (string, bool) {
	extClipOnce.Do(initExtClip)
	if xclipPath == "" {
		return "", false
	}
	cmd := exec.Command(xclipPath, "-selection", selection, "-o")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return "", false
	}
	out = bytes.TrimSuffix(out, []byte{0})
	return string(out), true
}

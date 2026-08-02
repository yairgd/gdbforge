package platform

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"

	"golang.design/x/clipboard"
)

var (
	clipOnce sync.Once
	clipOK   bool

	xclipOnce sync.Once
	xclipPath string
)

func initClipboard() {
	clipOK = clipboard.Init() == nil
}

func initXclip() {
	if p, err := exec.LookPath("xclip"); err == nil {
		xclipPath = p
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
		clipboard.Write(clipboard.FmtText, []byte(text))
		ok = true
	}
	// Also push CLIPBOARD via xclip when available (helps some terminals).
	if writeXSelection("clipboard", text) {
		ok = true
	}
	return ok
}

// SetPrimaryText copies text to the X11 PRIMARY selection (middle-click paste).
// golang.design/x/clipboard only owns CLIPBOARD; native terminals use PRIMARY
// for button-2 paste.
func SetPrimaryText(text string) bool {
	if text == "" {
		return false
	}
	return writeXSelection("primary", text)
}

// GetClipboardText reads the system CLIPBOARD text.
func GetClipboardText() (string, bool) {
	clipOnce.Do(initClipboard)
	if clipOK {
		b := clipboard.Read(clipboard.FmtText)
		if len(b) > 0 {
			return string(b), true
		}
	}
	return readXSelection("clipboard")
}

// GetPrimaryText reads the X11 PRIMARY selection (middle-click paste source).
func GetPrimaryText() (string, bool) {
	return readXSelection("primary")
}

func writeXSelection(selection, text string) bool {
	xclipOnce.Do(initXclip)
	if xclipPath == "" {
		return false
	}
	cmd := exec.Command(xclipPath, "-selection", selection, "-in")
	cmd.Stdin = strings.NewReader(text)
	// Detach stdio so a slow selection owner cannot block the UI thread.
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Wait() }()
	return true
}

func readXSelection(selection string) (string, bool) {
	xclipOnce.Do(initXclip)
	if xclipPath == "" {
		return "", false
	}
	cmd := exec.Command(xclipPath, "-selection", selection, "-o")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return "", false
	}
	// xclip may append a trailing newline when the selection had none;
	// keep bytes as-is except strip a single trailing NUL if present.
	out = bytes.TrimSuffix(out, []byte{0})
	return string(out), true
}

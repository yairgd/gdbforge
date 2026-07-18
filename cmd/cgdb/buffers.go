package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yairgd/cgdb-go/internal/cgdb/widgets"
)

// ensureCodeBuffer returns the CodeWidget for path, creating it if needed.
// PaneName is the file basename (Vim-style buffer name).
func (a *DebuggerApp) ensureCodeBuffer(path string) *widgets.CodeWidget {
	if path == "" {
		return nil
	}
	if a.fileBuffers == nil {
		a.fileBuffers = make(map[string]*widgets.CodeWidget)
	}
	if w, ok := a.fileBuffers[path]; ok {
		return w
	}
	w := widgets.NewCodeWidget()
	w.PaneName = filepath.Base(path)
	w.SetClipboard(a.ClipboardIO())
	if a.gdbWidget != nil {
		w.SetPTY(a.gdbWidget.Session(), a.State())
	}
	a.fileBuffers[path] = w
	return w
}

// findFileBuffer looks up an open file buffer by full path or basename.
func (a *DebuggerApp) findFileBuffer(name string) *widgets.CodeWidget {
	if name == "" || a.fileBuffers == nil {
		return nil
	}
	if w, ok := a.fileBuffers[name]; ok {
		return w
	}
	base := filepath.Base(name)
	for path, w := range a.fileBuffers {
		if path == name || filepath.Base(path) == name || filepath.Base(path) == base {
			return w
		}
	}
	return nil
}

// bufferCompletions returns dynamic :b Tab candidates (builtins + open file buffers).
func (a *DebuggerApp) bufferCompletions(prefix string) []string {
	seen := make(map[string]struct{})
	var names []string
	add := func(name string) {
		if name == "" {
			return
		}
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for name := range a.builtins {
		add(name)
	}
	for path, w := range a.fileBuffers {
		if w != nil && w.PaneName != "" {
			add(w.PaneName)
		}
		add(filepath.Base(path))
	}
	sort.Strings(names)
	return names
}

// resolveSourceFile maps a user argument to a readable path using SourceFiles, then disk.
func (a *DebuggerApp) resolveSourceFile(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}

	files := a.State().SourceFiles()
	for _, f := range files {
		if f == name {
			return f, true
		}
	}
	base := filepath.Base(name)
	var matches []string
	for _, f := range files {
		fb := filepath.Base(f)
		if fb == name || fb == base || f == name {
			matches = append(matches, f)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	if len(matches) > 1 {
		for _, f := range matches {
			if filepath.Base(f) == name {
				return f, true
			}
		}
		return matches[0], true
	}

	if _, err := os.Stat(name); err == nil {
		if abs, err := filepath.Abs(name); err == nil {
			return abs, true
		}
		return name, true
	}
	return "", false
}

func joinCmdArgs(args []any) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		s, ok := a.(string)
		if !ok || s == "" {
			continue
		}
		parts = append(parts, s)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// OnBuffer switches to an existing builtin or open file buffer (:b name).
// Does not create file buffers — use :e for that.
func (a *DebuggerApp) OnBuffer(args ...any) {
	name := joinCmdArgs(args)
	if name == "" || a.tab == nil {
		return
	}
	if w := a.builtins[name]; w != nil {
		if a.swapFocusedWidget(w) {
			a.RequestFrame()
		}
		return
	}
	if w := a.findFileBuffer(name); w != nil {
		if a.swapFocusedWidget(w) {
			a.RequestFrame()
		}
		return
	}
	if a.ctx.Log != nil {
		a.ctx.Log.Named("buffer").Error("no matching buffer: " + name)
	}
}

// OnEditFile opens (or reuses) a per-file CodeWidget and shows it (:e filename).
func (a *DebuggerApp) OnEditFile(args ...any) {
	name := joinCmdArgs(args)
	if name == "" || a.tab == nil {
		return
	}
	a.ensureSourceFiles()
	path, ok := a.resolveSourceFile(name)
	if !ok {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("edit").Error("file not found: " + name)
		}
		return
	}
	w := a.ensureCodeBuffer(path)
	if w == nil {
		return
	}
	line := 1
	if w.Path() == path && w.PCLine() > 0 {
		line = w.PCLine()
	}
	if err := w.ShowLocation(path, line); err != nil {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("edit").Error(err.Error())
		}
		return
	}
	a.scheduleBreakpointRefresh()
	_ = a.swapFocusedWidget(w)
	a.RequestFrame()
}

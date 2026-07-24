package main

import (
	"os"
	"sort"
	"strings"

	"path/filepath"

	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

// ensureCodeBuffer returns the CodeWidget for path, creating it if needed.
// PaneName is the file basename (Vim-style buffer name for :b); the status
// line shows the full path via CodeWidget.DrawStatusLine. created is true when
// a new widget was allocated (caller may need to paint breakpoint gutters).
func (a *DebuggerApp) ensureCodeBuffer(path string) (w *widgets.CodeWidget, created bool) {
	path = normalizeCodePath(path)
	if path == "" {
		return nil, false
	}
	if a.fileBuffers == nil {
		a.fileBuffers = make(map[string]*widgets.CodeWidget)
	}
	if existing, ok := a.fileBuffers[path]; ok {
		return existing, false
	}
	w = widgets.NewCodeWidget()
	w.PaneName = filepath.Base(path)
	w.SetClipboard(a.ClipboardIO())
	a.wireCodeWidget(w)
	a.fileBuffers[path] = w
	return w, true
}

// normalizeCodePath makes source paths absolute/clean so Delve's ./file.go and
// an absolute stop path share one CodeWidget buffer.
func normalizeCodePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "unavailable:") {
		return path
	}
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	return filepath.Clean(path)
}

// wireCodeWidget attaches app-owned breakpoint intents and mark colors.
func (a *DebuggerApp) wireCodeWidget(w *widgets.CodeWidget) {
	if w == nil {
		return
	}
	w.SetAppState(a.Debug())
	w.SetOnBreakToggle(a.onCodeBreakToggle)
	w.SetOnToggleEnable(a.toggleCodeBreakEnable)
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

// bufferCompletions returns :b Tab candidates: builtins + buffers the user
// opened with :edit / file-list. Auto-opened code from stop/stack/BP is kept
// out of the wildmenu (GDB SourceFiles includes deps like ldo.c / lapi.c).
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
	// Alias for the primary/active source pane (leaf mark "code").
	if cw := a.codeBufferForB(); cw != nil {
		add("code")
		if cw.PaneName != "" {
			add(cw.PaneName)
		}
		if path := cw.Path(); path != "" {
			add(filepath.Base(path))
		}
	}
	add("gdb")
	for path := range a.bufferListed {
		w := a.fileBuffers[path]
		if w == nil || w.Unavailable() {
			continue
		}
		if w.PaneName != "" {
			add(w.PaneName)
		}
		add(filepath.Base(path))
	}
	sort.Strings(names)
	return names
}

// markBufferListed adds path to :b Tab completions (explicit :edit / picker).
func (a *DebuggerApp) markBufferListed(path string) {
	if path == "" {
		return
	}
	if a.bufferListed == nil {
		a.bufferListed = make(map[string]struct{})
	}
	a.bufferListed[path] = struct{}{}
}

// resolveSourceFile maps a user argument to a readable path using SourceFiles, then disk.
func (a *DebuggerApp) resolveSourceFile(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}

	files := a.Debug().SourceFiles()
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

// codeBufferForB returns the CodeWidget for :b code / completions.
// Prefers primaryCode (stop / edit), then any open file buffer.
func (a *DebuggerApp) codeBufferForB() *widgets.CodeWidget {
	if a.primaryCode != nil && !a.primaryCode.Unavailable() {
		return a.primaryCode
	}
	for _, w := range a.fileBuffers {
		if w != nil && !w.Unavailable() {
			return w
		}
	}
	return nil
}

// OnBuffer switches to an existing builtin or open file buffer (:b name).
// Does not create file buffers — use :e for that.
func (a *DebuggerApp) OnBuffer(args ...any) {
	name := joinCmdArgs(args)
	if name == "" || a.tab == nil {
		return
	}
	// :b code → restore CodeWidget into the code leaf (not swap onto focused pane).
	if name == "code" {
		if cw := a.codeBufferForB(); cw != nil {
			a.placeCodeInSlot(cw)
			a.activateCodePane()
			a.RequestFrame()
			return
		}
		if a.ctx.Log != nil {
			a.ctx.Log.Named("buffer").Error("no code buffer open yet")
		}
		return
	}
	if name == "gdb" {
		a.activateGdbPane()
		a.RequestFrame()
		return
	}
	if w := a.builtins[name]; w != nil {
		if a.swapFocusedWidget(w) {
			a.maybeEnterLuaBuffer(w)
			a.RequestFrame()
		} else if _, ok := w.(*widgets.LuaWidget); ok {
			a.maybeEnterLuaBuffer(w)
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

// OnEdit opens the project file list (:edit) or a source file (:edit name).
// Unique prefix :e also resolves here (no separate :e leaf).
func (a *DebuggerApp) OnEdit(args ...any) {
	name := joinCmdArgs(args)
	if a.tab == nil {
		return
	}
	if name == "" {
		if a.fileListWidget == nil {
			return
		}
		a.ensureSourceFiles()
		a.syncFileListViews()
		if a.swapFocusedWidget(a.fileListWidget) {
			a.RequestFrame()
		}
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
	a.openSourcePath(path)
}

// openSourcePath shows path in a CodeWidget, replacing the focused pane
// (used by :edit <name> and FileListWidget selection).
func (a *DebuggerApp) openSourcePath(path string) {
	if path == "" || a.tab == nil {
		return
	}
	w, _ := a.ensureCodeBuffer(path)
	if w == nil {
		return
	}
	a.markBufferListed(path)
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
	a.onBreakpointsChanged()
	a.primaryCode = w
	if a.swapFocusedWidget(w) {
		if leaf := a.tab.FindLeaf(func(x termui.Widget) bool { return x == w }); leaf != nil {
			a.tab.SetLeafMark(leafMarkCode, leaf)
		}
		a.RequestFrame()
	}
}

// editCompletions returns dynamic :edit Tab candidates (SourceFiles full paths).
func (a *DebuggerApp) editCompletions(prefix string) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, f := range a.Debug().SourceFiles() {
		if f == "" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(f, prefix) && !strings.HasPrefix(filepath.Base(f), prefix) {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		names = append(names, f)
	}
	sort.Strings(names)
	return names
}

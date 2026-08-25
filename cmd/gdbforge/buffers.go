package main

import (
	"os"
	"sort"
	"strings"

	"path/filepath"

	"github.com/yairgd/gdbforge/internal/gdbforge/debugstate"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/luahost"
	"github.com/yairgd/gdbforge/internal/termui"
)

// bufferHost is the narrow surface bufferCtl needs from the composition root.
// DebuggerApp implements it; bufferCtl must not depend on *DebuggerApp.
type bufferHost interface {
	Debug() *debugstate.State
	Shell() *LayoutShell
	ClipboardIO() termui.ClipboardIO
	Builtins() map[string]termui.Widget
	FileListWidget() *widgets.FileListWidget
	ToggleCodeBreak(path string, line int)
	toggleCodeBreakEnable()
	BreakpointsChanged()
	PaintCodeBreaks(w *widgets.CodeWidget, path string)
	placeCodeInSlot(w *widgets.CodeWidget)
	OpenAssembly()
	SetPreferAsm(v bool)
	activateGdbPane()
	FocusCode()
	RequestFrame()
	ensureSourceFiles()
	syncFileListViews()
	swapFocusedWidget(w termui.Widget) bool
	LuaEnterBuffer(w termui.Widget)
	LuaEnsureBuffer(name string, from *luahost.Runtime) bool
	LogError(area, msg string)
}

// bufferCtl owns source-buffer state: the per-path CodeWidget map, the :b Tab
// list, the primary code pane, and :b / :edit buffer behavior.
// DebuggerApp wires it; the ctl owns the domain.
type bufferCtl struct {
	host bufferHost
	// files are per-path CodeWidgets opened via :e / GDB stop (PaneName = basename).
	files map[string]*widgets.CodeWidget
	// listed paths appear in :b Tab. Only :edit / FileList open marks them —
	// stop / callstack / BP preview must not pollute the wildmenu (ldo.c, …).
	listed map[string]struct{}
	// primary is the source pane last driven by a stop / :edit.
	primary *widgets.CodeWidget
}

// initMaps allocates the buffer registries at startup.
func (c *bufferCtl) initMaps() {
	c.files = make(map[string]*widgets.CodeWidget)
	c.listed = make(map[string]struct{})
}

// Buffers returns the live per-path CodeWidget map (nil before InitB).
func (c *bufferCtl) Buffers() map[string]*widgets.CodeWidget { return c.files }

// Primary returns the source pane a stop / :edit last drove.
func (c *bufferCtl) Primary() *widgets.CodeWidget {
	if c == nil {
		return nil
	}
	return c.primary
}

func (c *bufferCtl) setPrimary(w *widgets.CodeWidget) { c.primary = w }

// ensure returns the CodeWidget for path, creating it if needed.
// PaneName is the file basename (Vim-style buffer name for :b); the status
// line shows the full path via CodeWidget.DrawStatusLine. created is true when
// a new widget was allocated (caller may need to paint breakpoint gutters).
func (c *bufferCtl) ensure(path string) (w *widgets.CodeWidget, created bool) {
	path = normalizeCodePath(path)
	if path == "" {
		return nil, false
	}
	if c.files == nil {
		c.files = make(map[string]*widgets.CodeWidget)
	}
	if existing, ok := c.files[path]; ok {
		return existing, false
	}
	w = widgets.NewCodeWidget()
	w.PaneName = filepath.Base(path)
	if h := c.host; h != nil {
		w.SetClipboard(h.ClipboardIO())
	}
	c.wire(w)
	c.files[path] = w
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

// wire attaches host-owned breakpoint intents and mark colors.
func (c *bufferCtl) wire(w *widgets.CodeWidget) {
	h := c.host
	if w == nil || h == nil {
		return
	}
	w.SetAppState(h.Debug())
	w.SetOnBreakToggle(h.ToggleCodeBreak)
	w.SetOnToggleEnable(h.toggleCodeBreakEnable)
}

// find looks up an open file buffer by full path or basename.
func (c *bufferCtl) find(name string) *widgets.CodeWidget {
	if name == "" || c.files == nil {
		return nil
	}
	if w, ok := c.files[name]; ok {
		return w
	}
	base := filepath.Base(name)
	for path, w := range c.files {
		if path == name || filepath.Base(path) == name || filepath.Base(path) == base {
			return w
		}
	}
	return nil
}

// completions returns :b Tab candidates: builtins + buffers the user
// opened with :edit / file-list. Auto-opened code from stop/stack/BP is kept
// out of the wildmenu (GDB SourceFiles includes deps like ldo.c / lapi.c).
func (c *bufferCtl) completions(prefix string, _ bool) []string {
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
	if h := c.host; h != nil {
		for name := range h.Builtins() {
			add(name)
		}
	}
	// Alias for the primary/active source pane (leaf mark "code").
	if cw := c.codeBufferForB(); cw != nil {
		add("code")
		if cw.PaneName != "" {
			add(cw.PaneName)
		}
		if path := cw.Path(); path != "" {
			add(filepath.Base(path))
		}
	}
	add("gdb")
	for path := range c.listed {
		w := c.files[path]
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

// markListed adds path to :b Tab completions (explicit :edit / picker).
func (c *bufferCtl) markListed(path string) {
	if path == "" {
		return
	}
	if c.listed == nil {
		c.listed = make(map[string]struct{})
	}
	c.listed[path] = struct{}{}
}

// resolveSourceFile maps a user argument to a readable path using SourceFiles, then disk.
func (c *bufferCtl) resolveSourceFile(name string) (string, bool) {
	h := c.host
	name = strings.TrimSpace(name)
	if name == "" || h == nil {
		return "", false
	}

	files := h.Debug().SourceFiles()
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
// Prefers the primary pane (stop / edit), then any open file buffer.
func (c *bufferCtl) codeBufferForB() *widgets.CodeWidget {
	if c.primary != nil && !c.primary.Unavailable() {
		return c.primary
	}
	for _, w := range c.files {
		if w != nil && !w.Unavailable() {
			return w
		}
	}
	return nil
}

// onBuffer switches to an existing builtin or open file buffer (:b name).
// Pane scripts (snake, tetris, …) are created lazily on first :b.
func (c *bufferCtl) onBuffer(name string) {
	h := c.host
	if name == "" || h == nil || h.Shell().Tab() == nil {
		return
	}
	// :b code → restore CodeWidget into the code leaf (not swap onto focused pane).
	if name == "code" {
		h.SetPreferAsm(false)
		if cw := c.codeBufferForB(); cw != nil {
			h.placeCodeInSlot(cw)
			h.FocusCode()
			h.RequestFrame()
			return
		}
		h.LogError("buffer", "no code buffer open yet")
		return
	}
	if name == "asm" || name == "assembly" {
		h.OpenAssembly()
		return
	}
	if name == "gdb" {
		h.activateGdbPane()
		h.RequestFrame()
		return
	}
	c.openOrCreate(name, nil)
}

// onEdit opens the project file list (:edit) or a source file (:edit name).
// Unique prefix :e also resolves here (no separate :e leaf).
func (c *bufferCtl) onEdit(name string) {
	h := c.host
	if h == nil || h.Shell().Tab() == nil {
		return
	}
	if name == "" {
		fl := h.FileListWidget()
		if fl == nil {
			return
		}
		h.ensureSourceFiles()
		h.syncFileListViews()
		if h.swapFocusedWidget(fl) {
			h.RequestFrame()
		}
		return
	}
	h.ensureSourceFiles()
	path, ok := c.resolveSourceFile(name)
	if !ok {
		h.LogError("edit", "file not found: "+name)
		return
	}
	c.openSourcePath(path)
}

// openSourcePath shows path in a CodeWidget, replacing the focused pane
// (used by :edit <name> and FileListWidget selection).
func (c *bufferCtl) openSourcePath(path string) {
	h := c.host
	if path == "" || h == nil || h.Shell().Tab() == nil {
		return
	}
	w, _ := c.ensure(path)
	if w == nil {
		return
	}
	c.markListed(path)
	line := 1
	if w.Path() == path && w.PCLine() > 0 {
		line = w.PCLine()
	}
	if err := w.ShowLocation(path, line); err != nil {
		h.LogError("edit", err.Error())
		return
	}
	h.BreakpointsChanged()
	c.primary = w
	if h.swapFocusedWidget(w) {
		if leaf := h.Shell().Tab().FindLeaf(func(x termui.Widget) bool { return x == w }); leaf != nil {
			h.Shell().Tab().SetLeafMark(leafMarkCode, leaf)
		}
		h.RequestFrame()
	}
}

// editCompletions returns dynamic :edit Tab candidates (SourceFiles full paths).
func (c *bufferCtl) editCompletions(prefix string, _ bool) []string {
	h := c.host
	if h == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	for _, f := range h.Debug().SourceFiles() {
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

func (c *bufferCtl) focusBufferWidget(w termui.Widget) {
	h := c.host
	if w == nil || h == nil || h.Shell().Tab() == nil {
		return
	}
	if h.swapFocusedWidget(w) {
		h.LuaEnterBuffer(w)
		h.RequestFrame()
		return
	}
	if _, ok := w.(*widgets.LuaWidget); ok {
		h.LuaEnterBuffer(w)
		h.RequestFrame()
	}
}

// openOrCreate focuses an existing buffer, or creates a Lua pane when
// allowed: from a pane script's open_buffer, or lazy :b for known pane scripts.
func (c *bufferCtl) openOrCreate(name string, from *luahost.Runtime) {
	h := c.host
	if name == "" || h == nil || h.Shell().Tab() == nil {
		return
	}
	if w := h.Builtins()[name]; w != nil {
		c.focusBufferWidget(w)
		return
	}
	if w := c.find(name); w != nil {
		if h.swapFocusedWidget(w) {
			h.RequestFrame()
		}
		return
	}
	if from != nil {
		// Lua open_buffer: create only when the caller is a pane script.
		if from.HasPaneHooks() && h.LuaEnsureBuffer(name, from) {
			return
		}
		h.LogError("buffer", "no matching buffer: "+name)
		return
	}
	// :b name — only existing builtins/files (pane scripts appear after :lua creates them).
	h.LogError("buffer", "no matching buffer: "+name)
}

// showCodeAt loads file at line with ━━▶ (program counter).
func (c *bufferCtl) showCodeAt(file string, line int) *widgets.CodeWidget {
	return c.showCode(file, line, false)
}

// showCodeBrowse loads file and moves the blue code cursor without moving ━━▶.
func (c *bufferCtl) showCodeBrowse(file string, line int) *widgets.CodeWidget {
	return c.showCode(file, line, true)
}

// showCode loads file at line in a CodeWidget and paints BP gutters.
// browse=false moves ━━▶ (ShowLocation); browse=true only moves selection.
func (c *bufferCtl) showCode(file string, line int, browse bool) *widgets.CodeWidget {
	h := c.host
	if file == "" || h == nil {
		return nil
	}
	w, _ := c.ensure(file)
	if w == nil {
		return nil
	}
	if line < 1 {
		line = 1
	}
	if browse {
		_ = w.ShowSelection(file, line)
	} else {
		_ = w.ShowLocation(file, line)
	}
	h.Debug().SetCurrentLocation(file, line)
	if !w.Unavailable() {
		h.PaintCodeBreaks(w, file)
	}
	c.setPrimary(w)
	// Buffer update only — location leaf content is chosen by presentLocation.
	return w
}

// showCodeUnavailable shows a CodeWidget placeholder when there is no source path
// (e.g. ??? in libc) using func/detail as the displayed path line.
func (c *bufferCtl) showCodeUnavailable(label, extra string) *widgets.CodeWidget {
	h := c.host
	if h == nil {
		return nil
	}
	if label == "" {
		label = "(unknown)"
	}
	key := "unavailable:" + label
	w, _ := c.ensure(key)
	if w == nil {
		return nil
	}
	w.ShowUnavailable(label, extra)
	w.PaneName = filepath.Base(label)
	if w.PaneName == "" || w.PaneName == "." {
		w.PaneName = "unavailable"
	}
	c.setPrimary(w)
	// Buffer update only — presentLocation may show Asm instead of this banner.
	return w
}

// clearAll empties every source buffer and drops the primary pane
// (inferior exit / kill). Layout restore stays with DebuggerApp.
func (c *bufferCtl) clearAll() {
	seen := make(map[*widgets.CodeWidget]bool)
	for _, w := range c.files {
		if w == nil {
			continue
		}
		w.Clear()
		seen[w] = true
	}
	if c.primary != nil && !seen[c.primary] {
		c.primary.Clear()
	}
	c.primary = nil
}

// --- Host adapters (FileListHost / command trie need *DebuggerApp methods) ---

// OnBuffer switches to an existing builtin or open file buffer (:b name).
func (a *DebuggerApp) OnBuffer(args ...any) { a.bufs.onBuffer(joinCmdArgs(args)) }

// OnEdit opens the project file list (:edit) or a source file (:edit name).
func (a *DebuggerApp) OnEdit(args ...any) { a.bufs.onEdit(joinCmdArgs(args)) }

// OpenSourcePath shows path in a CodeWidget (FileListWidget selection).
func (a *DebuggerApp) OpenSourcePath(path string) { a.bufs.openSourcePath(path) }

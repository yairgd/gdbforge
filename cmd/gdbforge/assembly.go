package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/mcp"
	"github.com/yairgd/gdbforge/internal/termui"
)

type asmRefreshMsg struct {
	items   []models.AsmLine
	pcAddr  string
	selAddr string
	err     string
}

// asmHost is the narrow surface asmCtl needs from the composition root.
// DebuggerApp implements it; asmCtl must not depend on *DebuggerApp.
type asmHost interface {
	Backend() backend.Backend
	GdbMcp() *mcp.GdbMcpService
	Screen() tcell.Screen
	Workspace() *Workspace
	RequestFrame()
	RequestRedraw()
	FocusCode()
	findCodeLeaf() *termui.Node
	focusedLeaf() *termui.Node
	focusedWidget() termui.Widget
	isGdbLeaf(leaf *termui.Node) bool
	rememberCodeLeafFromFocus()
	CodeBufferForB() *widgets.CodeWidget
	PrimaryCode() *widgets.CodeWidget
	LogoWidget() *widgets.LogoWidget
	PaintAsmBreaks()
	LogError(area, msg string)
}

// asmCtl owns the disassembly domain: the Assembly model + view, the browse
// address refresh pipeline, and :b asm / :vs asm leaf policy.
// DebuggerApp wires it; the ctl owns the domain.
type asmCtl struct {
	host   asmHost
	widget *widgets.AssemblyWidget
	list   *models.AssemblyList
	// preferAsm means :b asm owns the code leaf until :b code (sticky user choice).
	preferAsm bool
	// autoAsm means the location leaf shows Assembly because the presented
	// frame has no readable source. Cleared when source returns or :b code.
	autoAsm bool
}

// Widget returns the shared AssemblyWidget (may be nil before InitB).
func (c *asmCtl) Widget() *widgets.AssemblyWidget { return c.widget }

// PreferAsm reports whether :b asm owns the code leaf.
func (c *asmCtl) PreferAsm() bool { return c != nil && c.preferAsm }

// AutoAsm reports whether Assembly was auto-opened for missing source.
func (c *asmCtl) AutoAsm() bool { return c != nil && c.autoAsm }

// setPreferAsm records whether Assembly keeps the code leaf (:b asm / :b code).
// Clearing preferAsm also clears autoAsm (:b code / prepare split).
func (c *asmCtl) setPreferAsm(v bool) {
	c.preferAsm = v
	if v {
		c.autoAsm = false
	} else {
		c.autoAsm = false
	}
}

func (c *asmCtl) setAutoAsm(v bool) {
	if c == nil {
		return
	}
	c.autoAsm = v
}

// ownsLocationLeaf reports whether Assembly should occupy the shared code leaf.
func (c *asmCtl) ownsLocationLeaf() bool {
	return c != nil && (c.preferAsm || c.autoAsm)
}

// supported reports whether the active backend can disassemble.
func (c *asmCtl) supported() bool {
	h := c.host
	return h != nil && h.Backend() != nil && h.Backend().SupportsAssembly()
}

// browse refetches disassembly centered on addr (widget edge/resize).
func (c *asmCtl) browse(addr string, rows int) {
	go c.runRefresh(addr, rows, false)
}

// armRefresh fetches disassembly around X on a background goroutine.
// When resetToPC is true, X is set to $pc; otherwise X stays on center (or
// the widget's current browse address when center is empty).
func (c *asmCtl) armRefresh(resetToPC bool) {
	h := c.host
	if !c.supported() || h.GdbMcp() == nil || c.widget == nil {
		return
	}
	center := ""
	if !resetToPC {
		center = c.widget.SelAddr()
	}
	go c.runRefresh(center, c.widget.VisibleRows(), resetToPC)
}

// armAround recenters Assembly browse on addr; ━━▶ stays on real $pc.
func (c *asmCtl) armAround(addr string) {
	h := c.host
	if !c.supported() || h.GdbMcp() == nil || c.widget == nil {
		return
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		c.armRefresh(false)
		return
	}
	go c.runRefresh(addr, c.widget.VisibleRows(), false)
}

// syncToFrame updates Assembly for a call-stack frame: level 0 → $pc,
// deeper frames → frame address (return / call site) as browse X.
func (c *asmCtl) syncToFrame(fr models.StackFrame) {
	if c == nil || !(c.hasSplit() || c.ownsLocationLeaf()) {
		return
	}
	if fr.Level == 0 {
		c.armRefresh(true)
		return
	}
	if fr.Addr != "" {
		c.armAround(fr.Addr)
	} else {
		c.armRefresh(false)
	}
}

func (c *asmCtl) runRefresh(center string, rows int, resetToPC bool) {
	items, pc, sel, err := c.queryAssembly(center, rows, resetToPC)
	msg := asmRefreshMsg{items: items, pcAddr: pc, selAddr: sel}
	if err != nil {
		msg.err = err.Error()
	}
	if h := c.host; h != nil {
		if scr := h.Screen(); scr != nil {
			_ = scr.PostEvent(tcell.NewEventInterrupt(msg))
		}
	}
}

func (c *asmCtl) queryAssembly(center string, rows int, resetToPC bool) (items []models.AsmLine, pcAddr, selAddr string, err error) {
	h := c.host
	if h == nil || h.GdbMcp() == nil {
		return nil, "", "", fmt.Errorf("no gdb session")
	}
	svc := h.GdbMcp()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pcRaw, qerr := svc.Query(ctx, "-data-evaluate-expression $pc")
	if qerr != nil {
		return nil, "", "", qerr
	}
	pc, ok := parse.ParseDataEvaluateAddr(pcRaw)
	if !ok {
		return nil, "", "", fmt.Errorf("cannot read $pc")
	}
	pcAddr = pc

	if resetToPC || center == "" {
		center = pc
	}
	selAddr = parse.NormalizeAddr(center)

	if rows < 1 {
		rows = parse.DefaultAsmRows
	}
	start, end, ok := parse.DisassembleRange(selAddr, rows)
	if !ok {
		return nil, pcAddr, selAddr, fmt.Errorf("bad address %s", selAddr)
	}
	cmd := fmt.Sprintf("-data-disassemble -s %s -e %s -- 1", start, end)
	raw, qerr := svc.Query(ctx, cmd)
	if qerr != nil {
		return nil, pcAddr, selAddr, qerr
	}
	all := parse.ParseDataDisassemble(raw)
	if len(all) == 0 {
		// Retry without opcodes (some stubs reject mode 1).
		cmd = fmt.Sprintf("-data-disassemble -s %s -e %s -- 0", start, end)
		raw, qerr = svc.Query(ctx, cmd)
		if qerr != nil {
			return nil, pcAddr, selAddr, qerr
		}
		all = parse.ParseDataDisassemble(raw)
	}
	if len(all) == 0 {
		return nil, pcAddr, selAddr, fmt.Errorf("empty disassembly")
	}
	items = parse.WindowAround(all, selAddr, rows)
	return items, pcAddr, selAddr, nil
}

func (c *asmCtl) applyRefresh(msg asmRefreshMsg) {
	h := c.host
	if c.widget == nil {
		return
	}
	if msg.err != "" {
		c.widget.ClearFetchAck()
		if h != nil {
			h.LogError("assembly", msg.err)
		}
		return
	}
	c.widget.SetItems(msg.items, msg.pcAddr, msg.selAddr)
	if c.list != nil {
		c.list.Set(msg.items, msg.pcAddr)
	}
	if h != nil {
		h.PaintAsmBreaks()
	}
	// Re-assert Assembly in its leaf after fetch (preferAsm, autoAsm, or split).
	c.placeInSlot(c.widget)
}

// placeInSlot updates the Assembly leaf without stealing Call Stack / Breakpoints.
// Shared code leaf: preferAsm (:b asm) or autoAsm (missing source).
func (c *asmCtl) placeInSlot(w *widgets.AssemblyWidget) {
	h := c.host
	if w == nil || h == nil || h.Workspace().Tab() == nil {
		return
	}
	tab := h.Workspace().Tab()
	// Dedicated :vs asm / :sp asm leaf — never touch the code leaf.
	if leaf := tab.LeafMark(leafMarkAsm); leaf != nil && !h.isGdbLeaf(leaf) {
		code := tab.LeafMark(leafMarkCode)
		if code == nil || code != leaf {
			leaf.SetWidget(w)
			tab.SetLeafMark(leafMarkAsm, leaf)
			return
		}
		// Mistaken asm mark on the shared location leaf — heal and fall through.
		tab.SetLeafMark(leafMarkAsm, nil)
	}
	// Shared location leaf: user sticky or auto for missing source.
	if !c.ownsLocationLeaf() {
		return
	}
	if !h.isGdbLeaf(h.focusedLeaf()) {
		if _, ok := h.focusedWidget().(*widgets.AssemblyWidget); ok {
			if h.focusedWidget() != w {
				_ = tab.ReplaceFocusedWidget(w)
			}
			h.rememberCodeLeafFromFocus()
			return
		}
		if isSourceCodeSlot(h.focusedWidget()) || isCodeSlot(h.focusedWidget()) {
			_ = tab.ReplaceFocusedWidget(w)
			h.rememberCodeLeafFromFocus()
			return
		}
	}
	if leaf := h.findCodeLeaf(); leaf != nil && !h.isGdbLeaf(leaf) {
		leaf.SetWidget(w)
		tab.SetLeafMark(leafMarkCode, leaf)
		return
	}
	if tab.ReplaceMatchingLeafWidget(w, isCodeSlot) {
		tab.SetLeafMark(leafMarkCode, tab.FindLeaf(isCodeSlot))
	}
}

// hasSplit reports a dedicated Assembly leaf from :vs asm / :sp asm.
// The shared location leaf showing Assembly (:b asm / autoAsm) is not a split.
func (c *asmCtl) hasSplit() bool {
	h := c.host
	if c == nil || h == nil || h.Workspace().Tab() == nil || c.widget == nil {
		return false
	}
	leaf := h.Workspace().Tab().LeafMark(leafMarkAsm)
	if leaf == nil || leaf.GetWidget() != c.widget {
		return false
	}
	code := h.Workspace().Tab().LeafMark(leafMarkCode)
	// Dedicated only when asm and code marks point at different leaves.
	return code != nil && code != leaf
}

// findLeaf returns the dedicated asm split leaf, if any.
func (c *asmCtl) findLeaf() *termui.Node {
	h := c.host
	if h == nil || h.Workspace().Tab() == nil {
		return nil
	}
	if leaf := h.Workspace().Tab().LeafMark(leafMarkAsm); leaf != nil && isAssemblyWidget(leaf.GetWidget()) {
		return leaf
	}
	return h.Workspace().Tab().FindLeaf(isAssemblyWidget)
}

// sourceUnavailable reports whether the Code buffer has no readable source.
func sourceUnavailable(codeW *widgets.CodeWidget) bool {
	return codeW == nil || codeW.Unavailable()
}

// shouldShow reports whether disassembly should refresh for this Code buffer.
func (c *asmCtl) shouldShow(codeW *widgets.CodeWidget) bool {
	if !c.supported() {
		return false
	}
	return c.hasSplit() || c.preferAsm || c.autoAsm || sourceUnavailable(codeW)
}

// openBuffer is :b asm / :b assembly — sticky user request for Assembly in the
// location leaf (unless :vs asm / :sp asm already has a dedicated leaf).
func (c *asmCtl) openBuffer() {
	h := c.host
	if h == nil {
		return
	}
	if !c.supported() {
		h.LogError("buffer", "assembly view is GDB-only for now")
		return
	}
	if c.widget == nil {
		return
	}
	if leaf := c.findLeaf(); leaf != nil && c.hasSplit() {
		_ = h.Workspace().Tab().FocusLeaf(leaf)
		c.armRefresh(true)
		h.RequestFrame()
		return
	}
	c.setPreferAsm(true)
	c.placeInSlot(c.widget)
	h.FocusCode()
	c.armRefresh(true)
	h.RequestFrame()
}

// prepareCodeForSplit restores source into the code leaf and focuses it.
func (c *asmCtl) prepareCodeForSplit() bool {
	h := c.host
	if h == nil || h.Workspace().Tab() == nil || c.widget == nil {
		return false
	}
	if !c.supported() {
		h.LogError("buffer", "assembly split is GDB-only for now")
		return false
	}
	c.setPreferAsm(false)
	cw := h.CodeBufferForB()
	if cw == nil {
		cw = h.PrimaryCode()
	}
	leaf := h.findCodeLeaf()
	if leaf != nil && isAssemblyWidget(leaf.GetWidget()) {
		if cw != nil {
			leaf.SetWidget(cw)
		} else if logo := h.LogoWidget(); logo != nil {
			leaf.SetWidget(logo)
		}
		h.Workspace().Tab().SetLeafMark(leafMarkCode, leaf)
	}
	h.FocusCode()
	return h.focusedLeaf() != nil && !h.isGdbLeaf(h.focusedLeaf())
}

// focusExistingSplit focuses the dedicated asm leaf if :vs asm / :sp asm
// already opened one. Returns true when a split already exists.
func (c *asmCtl) focusExistingSplit() bool {
	h := c.host
	if h == nil || !c.hasSplit() {
		return false
	}
	if leaf := c.findLeaf(); leaf != nil {
		_ = h.Workspace().Tab().FocusLeaf(leaf)
	}
	c.armRefresh(true)
	h.RequestRedraw()
	return true
}

// splitAsm opens a dedicated asm split (horizontal=true → below, else right).
func (c *asmCtl) splitAsm(horizontal bool) {
	h := c.host
	if h == nil {
		return
	}
	if c.focusExistingSplit() {
		return
	}
	if !c.prepareCodeForSplit() {
		return
	}
	tab := h.Workspace().Tab()
	if horizontal {
		tab.HorizontalSplit(c.widget)
	} else {
		tab.VerticalSplit(c.widget)
	}
	codeLeaf := h.focusedLeaf()
	asmLeaf := tab.FindLeaf(func(w termui.Widget) bool { return w == c.widget })
	if codeLeaf != nil {
		tab.SetLeafMark(leafMarkCode, codeLeaf)
	}
	if asmLeaf != nil {
		tab.SetLeafMark(leafMarkAsm, asmLeaf)
	}
	c.armRefresh(true)
	h.RequestRedraw()
}

func isAssemblyWidget(w termui.Widget) bool {
	_, ok := w.(*widgets.AssemblyWidget)
	return ok
}

// --- Host adapters (AssemblyHost / command trie need *DebuggerApp methods) ---

// BrowseAssembly refetches disassembly centered on addr (widget edge/resize).
func (a *DebuggerApp) BrowseAssembly(addr string, rows int) { a.asm.browse(addr, rows) }

// SplitAsmBelow is :sp asm / :split asm — Code on top, Assembly below.
func (a *DebuggerApp) SplitAsmBelow(args ...any) { a.asm.splitAsm(true) }

// SplitAsmRight is :vs asm / :vsplit asm — Code on the left, Assembly on the right.
func (a *DebuggerApp) SplitAsmRight(args ...any) { a.asm.splitAsm(false) }

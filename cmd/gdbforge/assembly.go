package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tcell "github.com/gdamore/tcell/v2"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
	"github.com/yairgd/gdbforge/internal/gdbforge/widgets"
	"github.com/yairgd/gdbforge/internal/termui"
)

type asmRefreshMsg struct {
	items   []models.AsmLine
	pcAddr  string
	selAddr string
	err     string
}

// armAssemblyRefresh fetches disassembly around X on a background goroutine.
// When resetToPC is true, X is set to $pc; otherwise X stays on center (or
// the widget's current browse address when center is empty).
func (a *DebuggerApp) armAssemblyRefresh(resetToPC bool) {
	if a == nil || a.isDLV() || a.gdbMcp == nil {
		return
	}
	if a.assemblyWidget == nil {
		return
	}
	center := ""
	if !resetToPC && a.assemblyWidget != nil {
		center = a.assemblyWidget.SelAddr()
	}
	rows := 0
	if a.assemblyWidget != nil {
		rows = a.assemblyWidget.VisibleRows()
	}
	go a.runAssemblyRefresh(center, rows, resetToPC)
}

// armAssemblyAround recenters Assembly browse on addr; ━━▶ stays on real $pc.
func (a *DebuggerApp) armAssemblyAround(addr string) {
	if a == nil || a.isDLV() || a.gdbMcp == nil || a.assemblyWidget == nil {
		return
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		a.armAssemblyRefresh(false)
		return
	}
	rows := a.assemblyWidget.VisibleRows()
	go a.runAssemblyRefresh(addr, rows, false)
}

// syncAssemblyToFrame updates Assembly for a call-stack frame: level 0 → $pc,
// deeper frames → frame address (return / call site) as browse X.
func (a *DebuggerApp) syncAssemblyToFrame(fr models.StackFrame) {
	if a == nil || !(a.hasAsmSplit() || a.preferAsm) {
		return
	}
	if fr.Level == 0 {
		a.armAssemblyRefresh(true)
		return
	}
	if fr.Addr != "" {
		a.armAssemblyAround(fr.Addr)
	}
}

func (a *DebuggerApp) runAssemblyRefresh(center string, rows int, resetToPC bool) {
	items, pc, sel, err := a.queryAssembly(center, rows, resetToPC)
	msg := asmRefreshMsg{items: items, pcAddr: pc, selAddr: sel}
	if err != nil {
		msg.err = err.Error()
	}
	if scr := a.Screen(); scr != nil {
		_ = scr.PostEvent(tcell.NewEventInterrupt(msg))
	}
}

func (a *DebuggerApp) queryAssembly(center string, rows int, resetToPC bool) (items []models.AsmLine, pcAddr, selAddr string, err error) {
	if a.gdbMcp == nil {
		return nil, "", "", fmt.Errorf("no gdb session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pcRaw, qerr := a.gdbMcp.Query(ctx, "-data-evaluate-expression $pc")
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
	raw, qerr := a.gdbMcp.Query(ctx, cmd)
	if qerr != nil {
		return nil, pcAddr, selAddr, qerr
	}
	all := parse.ParseDataDisassemble(raw)
	if len(all) == 0 {
		// Retry without opcodes (some stubs reject mode 1).
		cmd = fmt.Sprintf("-data-disassemble -s %s -e %s -- 0", start, end)
		raw, qerr = a.gdbMcp.Query(ctx, cmd)
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

func (a *DebuggerApp) applyAsmRefresh(msg asmRefreshMsg) {
	if a.assemblyWidget == nil {
		return
	}
	if msg.err != "" {
		a.assemblyWidget.ClearFetchAck()
		if a.ctx.Log != nil {
			a.ctx.Log.Named("assembly").Error(msg.err)
		}
		return
	}
	a.assemblyWidget.SetItems(msg.items, msg.pcAddr, msg.selAddr)
	if a.assembly != nil {
		a.assembly.Set(msg.items, msg.pcAddr)
	}
	if a.breakpoints != nil {
		a.assemblyWidget.SetBreakInfos(a.breakpoints.Items())
	}
	// Layout changes only via :b asm / :vs asm / :sp asm — never auto-swap with Code.
	a.placeAsmInSlot(a.assemblyWidget)
}

// placeAsmInSlot updates the Assembly leaf without stealing a Code pane.
// Code ↔ Assembly in the shared code leaf only when preferAsm (:b asm).
func (a *DebuggerApp) placeAsmInSlot(w *widgets.AssemblyWidget) {
	if w == nil || a.tab == nil {
		return
	}
	// Dedicated :vs asm / :sp asm leaf — never touch the code leaf.
	if leaf := a.tab.LeafMark(leafMarkAsm); leaf != nil && !a.isGdbLeaf(leaf) {
		leaf.SetWidget(w)
		a.tab.SetLeafMark(leafMarkAsm, leaf)
		return
	}
	// Single-pane: only replace Code when the user asked :b asm.
	if !a.preferAsm {
		return
	}
	if !a.isGdbLeaf(a.focusedLeaf()) {
		if _, ok := a.focusedWidget().(*widgets.AssemblyWidget); ok {
			if a.focusedWidget() != w {
				_ = a.tab.ReplaceFocusedWidget(w)
			}
			a.rememberCodeLeafFromFocus()
			return
		}
		if isSourceCodeSlot(a.focusedWidget()) || isCodeSlot(a.focusedWidget()) {
			_ = a.tab.ReplaceFocusedWidget(w)
			a.rememberCodeLeafFromFocus()
			return
		}
	}
	if leaf := a.findCodeLeaf(); leaf != nil && !a.isGdbLeaf(leaf) {
		leaf.SetWidget(w)
		a.tab.SetLeafMark(leafMarkCode, leaf)
		return
	}
	if a.tab.ReplaceMatchingLeafWidget(w, isCodeSlot) {
		a.tab.SetLeafMark(leafMarkCode, a.tab.FindLeaf(isCodeSlot))
	}
}

// shouldShowAssembly reports whether disassembly should refresh.
// Never auto-opens asm for missing source — only :b asm or :vs asm / :sp asm.
func (a *DebuggerApp) shouldShowAssembly(codeW *widgets.CodeWidget) bool {
	if a == nil || a.isDLV() {
		return false
	}
	_ = codeW
	return a.hasAsmSplit() || a.preferAsm
}

// openAssemblyBuffer is :b asm / :b assembly — the only request that puts
// Assembly into the code leaf (unless :vs asm / :sp asm already has a dedicated leaf).
func (a *DebuggerApp) openAssemblyBuffer() {
	if a.isDLV() {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("buffer").Error("assembly view is GDB-only for now")
		}
		return
	}
	if a.assemblyWidget == nil {
		return
	}
	if leaf := a.findAsmLeaf(); leaf != nil && a.hasAsmSplit() {
		_ = a.tab.FocusLeaf(leaf)
		a.armAssemblyRefresh(true)
		a.RequestFrame()
		return
	}
	a.preferAsm = true
	a.placeAsmInSlot(a.assemblyWidget)
	a.activateCodePane()
	a.armAssemblyRefresh(true)
	a.RequestFrame()
}

// prepareCodeForAsmSplit restores source into the code leaf and focuses it.
func (a *DebuggerApp) prepareCodeForAsmSplit() bool {
	if a == nil || a.tab == nil || a.assemblyWidget == nil {
		return false
	}
	if a.isDLV() {
		if a.ctx.Log != nil {
			a.ctx.Log.Named("buffer").Error("assembly split is GDB-only for now")
		}
		return false
	}
	a.preferAsm = false
	cw := a.codeBufferForB()
	if cw == nil {
		cw = a.primaryCode
	}
	leaf := a.findCodeLeaf()
	if leaf != nil && isAssemblyWidget(leaf.GetWidget()) {
		if cw != nil {
			leaf.SetWidget(cw)
		} else if a.logoWidget != nil {
			leaf.SetWidget(a.logoWidget)
		}
		a.tab.SetLeafMark(leafMarkCode, leaf)
	}
	a.activateCodePane()
	return a.focusedLeaf() != nil && !a.isGdbLeaf(a.focusedLeaf())
}

// focusExistingAsmSplit focuses the dedicated asm leaf if :vs asm / :sp asm
// already opened one. Returns true when a split already exists.
func (a *DebuggerApp) focusExistingAsmSplit() bool {
	if !a.hasAsmSplit() {
		return false
	}
	if leaf := a.findAsmLeaf(); leaf != nil {
		_ = a.tab.FocusLeaf(leaf)
	}
	a.armAssemblyRefresh(true)
	a.RequestRedraw()
	return true
}

// SplitAsmBelow is :sp asm / :split asm — Code on top, Assembly below.
// Only one asm split is allowed; a second :sp asm / :vs asm just focuses it.
func (a *DebuggerApp) SplitAsmBelow(args ...any) {
	if a.focusExistingAsmSplit() {
		return
	}
	if !a.prepareCodeForAsmSplit() {
		return
	}
	a.tab.HorizontalSplit(a.assemblyWidget)
	codeLeaf := a.focusedLeaf()
	asmLeaf := a.tab.FindLeaf(func(w termui.Widget) bool { return w == a.assemblyWidget })
	if codeLeaf != nil {
		a.tab.SetLeafMark(leafMarkCode, codeLeaf)
	}
	if asmLeaf != nil {
		a.tab.SetLeafMark(leafMarkAsm, asmLeaf)
	}
	a.armAssemblyRefresh(true)
	a.RequestRedraw()
}

// SplitAsmRight is :vs asm / :vsplit asm — Code on the left, Assembly on the right.
// Only one asm split is allowed; a second :vs asm / :sp asm just focuses it.
func (a *DebuggerApp) SplitAsmRight(args ...any) {
	if a.focusExistingAsmSplit() {
		return
	}
	if !a.prepareCodeForAsmSplit() {
		return
	}
	a.tab.VerticalSplit(a.assemblyWidget)
	// After VerticalSplit: First=code (focus), Second=asm (right).
	codeLeaf := a.focusedLeaf()
	asmLeaf := a.tab.FindLeaf(func(w termui.Widget) bool { return w == a.assemblyWidget })
	if codeLeaf != nil {
		a.tab.SetLeafMark(leafMarkCode, codeLeaf)
	}
	if asmLeaf != nil {
		a.tab.SetLeafMark(leafMarkAsm, asmLeaf)
	}
	a.armAssemblyRefresh(true)
	a.RequestRedraw()
}

func isAssemblyWidget(w termui.Widget) bool {
	_, ok := w.(*widgets.AssemblyWidget)
	return ok
}

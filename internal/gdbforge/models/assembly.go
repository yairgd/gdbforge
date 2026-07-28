package models

// AsmLine is one disassembled instruction from -data-disassemble.
type AsmLine struct {
	Addr    string // e.g. "0x401126"
	Opcodes string // optional hex bytes
	Inst    string // mnemonic + operands
	Func    string // optional func-name from MI
	Offset  string // optional offset within func
}

// AssemblyList is the shared disassembly snapshot for the AssemblyWidget.
type AssemblyList struct {
	items  []AsmLine
	pcAddr string
}

// Set replaces the instruction list and remembered $pc address.
func (a *AssemblyList) Set(items []AsmLine, pcAddr string) {
	if a == nil {
		return
	}
	a.items = append([]AsmLine(nil), items...)
	a.pcAddr = pcAddr
}

// Items returns a copy of the current instructions.
func (a *AssemblyList) Items() []AsmLine {
	if a == nil || len(a.items) == 0 {
		return nil
	}
	return append([]AsmLine(nil), a.items...)
}

// PCAddr returns the last $pc address associated with this list.
func (a *AssemblyList) PCAddr() string {
	if a == nil {
		return ""
	}
	return a.pcAddr
}

// Len returns the number of instructions.
func (a *AssemblyList) Len() int {
	if a == nil {
		return 0
	}
	return len(a.items)
}

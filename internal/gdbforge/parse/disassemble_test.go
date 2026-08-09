package parse

import (
	"testing"

	"github.com/yairgd/gdbforge/internal/gdbforge/models"
)

func TestParseDataDisassemble(t *testing.T) {
	raw := `^done,asm_insns=[{address="0x0000000000401126",func-name="main",offset="22",opcodes="89 7d fc",inst="mov    %edi,-0x4(%rbp)"},{address="0x0000000000401129",func-name="main",offset="25",opcodes="89 75 f8",inst="mov    %esi,-0x8(%rbp)"}]`
	got := ParseDataDisassemble(raw)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(got), got)
	}
	if got[0].Addr != "0x401126" {
		t.Fatalf("addr0=%q", got[0].Addr)
	}
	if got[0].Opcodes != "89 7d fc" {
		t.Fatalf("opcodes0=%q", got[0].Opcodes)
	}
	if got[0].Inst != "mov    %edi,-0x4(%rbp)" {
		t.Fatalf("inst0=%q", got[0].Inst)
	}
	if got[1].Addr != "0x401129" {
		t.Fatalf("addr1=%q", got[1].Addr)
	}
}

func TestParseDataEvaluateAddr(t *testing.T) {
	addr, ok := ParseDataEvaluateAddr(`^done,value="0x401126"`)
	if !ok || addr != "0x401126" {
		t.Fatalf("got %q ok=%v", addr, ok)
	}
	addr, ok = ParseDataEvaluateAddr(`^done,value="(void *) 0x7fffffff"`)
	if !ok || addr != "0x7fffffff" {
		t.Fatalf("got %q ok=%v", addr, ok)
	}
}

func TestWindowAroundCentered(t *testing.T) {
	all := make([]models.AsmLine, 20)
	for i := range all {
		all[i] = models.AsmLine{
			Addr: NormalizeAddr(sprintfAddr(0x1000 + uint64(i)*4)),
			Inst: "nop",
		}
	}
	got := WindowAround(all, "0x1028", 5)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5", len(got))
	}
	// 0x1028 is index 10 (0x1000+10*4); window center at index 2 of 5.
	if got[2].Addr != "0x1028" {
		t.Fatalf("center=%q want 0x1028", got[2].Addr)
	}
}

func TestWindowAroundPCStartsAtPC(t *testing.T) {
	all := make([]models.AsmLine, 20)
	for i := range all {
		all[i] = models.AsmLine{
			Addr: NormalizeAddr(sprintfAddr(0x1000 + uint64(i)*4)),
			Inst: "nop",
		}
	}
	got := WindowAroundPC(all, "0x1028", 5)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5", len(got))
	}
	// CGDB x/Ni $pc: first line is the PC.
	if got[0].Addr != "0x1028" {
		t.Fatalf("pc at %q want 0x1028 (first line)", got[0].Addr)
	}
}

func TestWindowAroundPCNearEndKeepsPCFirst(t *testing.T) {
	all := make([]models.AsmLine, 12)
	for i := range all {
		all[i] = models.AsmLine{
			Addr: NormalizeAddr(sprintfAddr(0x1000 + uint64(i)*4)),
			Inst: "nop",
		}
	}
	// Only 2 insns after 0x1028 — must not rewind to fill a 10-row window.
	got := WindowAroundPC(all, "0x1028", 10)
	if got[0].Addr != "0x1028" {
		t.Fatalf("first=%q want 0x1028", got[0].Addr)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (only remaining after PC)", len(got))
	}
}

func TestWindowAtAnchorKeepsCenterRow(t *testing.T) {
	all := make([]models.AsmLine, 40)
	for i := range all {
		all[i] = models.AsmLine{
			Addr: NormalizeAddr(sprintfAddr(0x1000 + uint64(i)*4)),
			Inst: "nop",
		}
	}
	got := WindowAtAnchor(all, "0x1028", 20, 5)
	if len(got) != 20 {
		t.Fatalf("len=%d want 20", len(got))
	}
	if got[5].Addr != "0x1028" {
		t.Fatalf("anchor=%q want 0x1028 at index 5", got[5].Addr)
	}
}

func TestBrowseAnchor(t *testing.T) {
	// Down with caret near bottom of a 20-row view → small index (room below).
	if a := BrowseAnchor(1, 18, 20, 80); a != 18 {
		t.Fatalf("down anchor=%d want 18", a)
	}
	// Up with caret near top → large index (room above).
	if a := BrowseAnchor(-1, 2, 20, 80); a != 80-1-(20-2-1) {
		t.Fatalf("up anchor=%d", a)
	}
}

func TestWindowBeforeEndsAtCenter(t *testing.T) {
	all := make([]models.AsmLine, 20)
	for i := range all {
		all[i] = models.AsmLine{
			Addr: NormalizeAddr(sprintfAddr(0x1000 + uint64(i)*4)),
			Inst: "nop",
		}
	}
	got := WindowBefore(all, "0x1028", 5)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5", len(got))
	}
	if got[len(got)-1].Addr != "0x1028" {
		t.Fatalf("last=%q want 0x1028", got[len(got)-1].Addr)
	}
}

func TestDisassembleRangeForward(t *testing.T) {
	start, end, ok := DisassembleRangeForward("0x1000", 40)
	if !ok {
		t.Fatal("expected ok")
	}
	s, _ := ParseAddrUint(start)
	e, _ := ParseAddrUint(end)
	if s >= 0x1000 {
		t.Fatalf("start %s should be before center", start)
	}
	if e <= 0x1000+uint64(40*maxInsnBytes) {
		t.Fatalf("end %s too close; want forward bias", end)
	}
}

func TestDisassembleRange(t *testing.T) {
	start, end, ok := DisassembleRange("0x1000", 10)
	if !ok {
		t.Fatal("expected ok")
	}
	s, _ := ParseAddrUint(start)
	e, _ := ParseAddrUint(end)
	if s >= 0x1000 || e <= 0x1000 {
		t.Fatalf("range %s..%s does not span center", start, end)
	}
}

func sprintfAddr(n uint64) string {
	return NormalizeAddr("0x" + toHex(n))
}

func toHex(n uint64) string {
	const hexdigits = "0123456789abcdef"
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = hexdigits[n&0xf]
		n >>= 4
	}
	return string(buf[i:])
}

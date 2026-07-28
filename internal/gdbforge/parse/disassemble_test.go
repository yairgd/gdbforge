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

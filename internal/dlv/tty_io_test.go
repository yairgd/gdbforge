package dlv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInferiorStdoutOnTTY(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("no dlv")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	prog := filepath.Join(dir, "prog")
	if err := os.WriteFile(src, []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"TTYHELLO\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", prog, src)
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("build: %v %s", err, out)
	}
	c, err := NewClient("dlv", []string{prog})
	if err != nil {
		t.Skipf("dlv: %v", err)
	}
	defer c.Close()
	inf := c.InferiorTTY()
	if inf == nil {
		t.Fatal("no inferior tty")
	}
	ch, cancel := inf.Subscribe()
	defer cancel()

	_ = c.Send("break main.main")
	time.Sleep(300 * time.Millisecond)
	_ = c.Send("continue")
	time.Sleep(300 * time.Millisecond)
	_ = c.Send("continue")

	deadline := time.After(8 * time.Second)
	var got strings.Builder
	for {
		select {
		case <-deadline:
			t.Fatalf("no inferior output; got %q slave=%s", got.String(), inf.SlaveName())
		case msg, ok := <-ch:
			if !ok {
				t.Fatalf("closed; got %q", got.String())
			}
			got.WriteString(msg.Data)
			if strings.Contains(got.String(), "TTYHELLO") {
				return
			}
		}
	}
}

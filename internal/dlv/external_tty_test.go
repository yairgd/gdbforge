package dlv

import (
	"os"
	"os/exec"
	"testing"

	"github.com/yairgd/gdbforge/internal/ptyx"
)

func TestNewClientOptsExternalInferiorOwnedPTY(t *testing.T) {
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("no dlv")
	}
	prog := "../../hello-go"
	if _, err := os.Stat(prog); err != nil {
		t.Skip("no hello-go")
	}
	inf, err := ptyx.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer inf.Close()

	client, err := NewClientOpts("dlv", []string{prog}, ClientOptions{InferiorTTY: inf.SlaveName()})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if !client.UsesExternalInferiorTTY() {
		t.Fatalf("expected external inferior, path=%q rpc=%v", client.InferiorTTYPath(), client.RPC != nil)
	}
	if client.RPC == nil {
		t.Fatal("expected rpc2 client for headless external tty")
	}
}

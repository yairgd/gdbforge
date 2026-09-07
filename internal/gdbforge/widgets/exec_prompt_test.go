package widgets

import (
	"testing"

	tcell "github.com/gdamore/tcell/v2"
)

func TestExecDismissAfterSessionEnded(t *testing.T) {
	w := NewExecWidget()
	dismissed := false
	w.SetOnDismiss(func() { dismissed = true })

	w.HandleEvent(tcell.NewEventInterrupt(execSessionEnded))
	if !w.Ended() {
		t.Fatal("expected ended after session-ended interrupt")
	}

	w.HandleEvent(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if !dismissed {
		t.Fatal("expected dismiss on key after exit")
	}
}

package core

// Confirming is the shared quit/yes-no gate surface (gdb.QuitGate, dlv.ConfirmGate).
type Confirming interface {
	Confirming() bool
}

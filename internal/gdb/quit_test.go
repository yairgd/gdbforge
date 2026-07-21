package gdb

import "testing"

func TestQuitGateRequestQuitConfirm(t *testing.T) {
	var g QuitGate
	g.Observe(MiUpdate{InferiorPID: "99"})
	if g.RequestQuit() != QuitShowConfirm || !g.Confirming() {
		t.Fatalf("alive RequestQuit: confirming=%v", g.Confirming())
	}
	if g.RequestQuit() != QuitSendQ {
		t.Fatal("second RequestQuit while confirming should send q")
	}
}

func TestQuitGateRequestQuitNoInferior(t *testing.T) {
	var g QuitGate
	if g.RequestQuit() != QuitSendQ {
		t.Fatal("no inferior should SendQ")
	}
}

func TestQuitGateSubmitAndAnswer(t *testing.T) {
	var g QuitGate
	g.Observe(MiUpdate{InferiorPID: "1"})
	if g.SubmitQuitCommand("break main") != QuitNoop {
		t.Fatal("non-quit should be Noop")
	}
	if g.SubmitQuitCommand("q") != QuitShowConfirm {
		t.Fatal("q should confirm")
	}
	if g.Answer("maybe") != QuitReprompt || !g.Confirming() {
		t.Fatal("bad answer should reprompt")
	}
	if g.Answer("n") != QuitSendEmpty || g.Confirming() {
		t.Fatal("n should cancel")
	}
	g.Observe(MiUpdate{InferiorPID: "1"})
	_ = g.SubmitQuitCommand("quit")
	if g.Answer("y") != QuitSendQuit {
		t.Fatal("y should SendQuit")
	}
}

func TestQuitGateObserveExit(t *testing.T) {
	var g QuitGate
	g.Observe(MiUpdate{InferiorPID: "1"})
	g.confirming = true
	g.Observe(MiUpdate{InferiorExited: true})
	if g.InferiorAlive() || g.Confirming() {
		t.Fatal("exit should clear alive/confirm")
	}
}

func TestIsQuitCmd(t *testing.T) {
	if !IsQuitCmd("q") || !IsQuitCmd("Quit") || IsQuitCmd("break") {
		t.Fatal("IsQuitCmd mismatch")
	}
}

package termui

import (
	"errors"
	"testing"
)

func TestIsAlreadyEngaged(t *testing.T) {
	if !isAlreadyEngaged(errors.New("already engaged")) {
		t.Fatal("expected match")
	}
	if isAlreadyEngaged(errors.New("other error")) {
		t.Fatal("unexpected match")
	}
	if isAlreadyEngaged(nil) {
		t.Fatal("nil should not match")
	}
}

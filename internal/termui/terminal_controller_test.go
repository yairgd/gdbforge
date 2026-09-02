package termui

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTerminalControllerWriteAndCell(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	err := c.WriteString("hello")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	cell := c.Cell(0, 0)
	if cell.Rune != 'h' {
		t.Fatalf("expected 'h', got %q", cell.Rune)
	}

	cell = c.Cell(4, 0)
	if cell.Rune != 'o' {
		t.Fatalf("expected 'o', got %q", cell.Rune)
	}
}

func TestTerminalControllerColor(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	err := c.WriteString("\x1b[32mX\x1b[0m")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	cell := c.Cell(0, 0)

	fg, _, _ := cell.Style.Decompose()

	if fg == tcell.ColorDefault {
		t.Fatal("expected non-default foreground color")
	}
}

func TestTerminalControllerCursor(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	err := c.WriteString("abc")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	x, y := c.Cursor()

	if x != 3 || y != 0 {
		t.Fatalf("expected cursor 3,0 got %d,%d", x, y)
	}
}

func TestTerminalControllerResize(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	err := c.Resize(120, 40)
	if err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	cols, rows := c.Size()

	if cols != 120 || rows != 40 {
		t.Fatalf(
			"expected 120x40, got %dx%d",
			cols,
			rows,
		)
	}
}

func TestTerminalControllerInputHandler(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	var received bytes.Buffer

	c.SetInputHandler(func(data []byte) error {
		_, err := received.Write(data)
		return err
	})

	err := c.SendString("hello")
	if err != nil {
		t.Fatalf("SendString failed: %v", err)
	}

	if received.String() != "hello" {
		t.Fatalf(
			"expected input handler to receive %q, got %q",
			"hello",
			received.String(),
		)
	}
}

func TestTerminalControllerClosed(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)

	c.Close()

	err := c.WriteString("hello")

	if err == nil {
		t.Fatal("expected error after Close")
	}

	cols, rows := c.Size()

	if cols != 0 || rows != 0 {
		t.Fatalf(
			"expected size 0x0 after Close, got %dx%d",
			cols,
			rows,
		)
	}
}

func TestTerminalControllerANSIPositioning(t *testing.T) {
	c := NewTerminalController(80, 24, 1000)
	defer c.Close()

	// ANSI cursor position:
	// ESC [ row ; col H
	//
	// ANSI coordinates are 1-based.
	err := c.WriteString("\x1b[5;10HX")
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	cell := c.Cell(9, 4)

	if cell.Rune != 'X' {
		t.Fatalf(
			"expected X at 9,4, got %q",
			cell.Rune,
		)
	}
}

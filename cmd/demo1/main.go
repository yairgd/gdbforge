package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
	xterm "github.com/gitpod-io/xterm-go"
)

func main() {
	const (
		cols = 80
		rows = 24
	)

	//
	// 1. Create headless terminal emulator.
	//
	term := xterm.New(
		xterm.WithCols(cols),
		xterm.WithRows(rows),
		xterm.WithScrollback(1000),
	)
	defer term.Dispose()

	//
	// 2. Start C application attached to a real PTY.
	//
	cmd := exec.Command("./demo")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer ptmx.Close()

	//
	// 3. PTY output -> terminal emulator.
	//
	done := make(chan struct{})

	go func() {
		defer close(done)

		buf := make([]byte, 4096)

		for {
			n, err := ptmx.Read(buf)

			if n > 0 {
				term.Write(buf[:n])
			}

			if err != nil {
				if errors.Is(err, syscall.EIO) ||
					errors.Is(err, io.EOF) {
					return
				}

				fmt.Printf("PTY read error: %v\n", err)
				return
			}
		}
	}()
	//
	// Give program time to reach fgets().
	//
	time.Sleep(200 * time.Millisecond)

	printScreen(term)

	//
	// 4. Send keyboard input to the application.
	//
	_, _ = ptmx.Write([]byte("Yair\r"))

	time.Sleep(200 * time.Millisecond)

	printScreen(term)

	<-done
}

func printScreen(term *xterm.Terminal) {
	fmt.Println("----- terminal screen -----")
	fmt.Println(term.String())
	fmt.Printf(
		"cursor = %d,%d\n",
		term.CursorX(),
		term.CursorY(),
	)
}

/*
for y := 0; y < rows; y++ {
    for x := 0; x < cols; x++ {
        cell := /* get xterm cell

        ch := cell.Char()
        fg := cell.Foreground()
        bg := cell.Background()

        grid.SetCell(x, y, ch, fg, bg)
    }
}
*/

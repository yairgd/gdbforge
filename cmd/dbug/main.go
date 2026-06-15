package main

import (
	tea "github.com/charmbracelet/bubbletea"
	//	"github.com/yairgd/cgdb-go/internal/core"
	"github.com/yairgd/cgdb-go/internal/gdb"
	"github.com/yairgd/cgdb-go/internal/ui/tui"

	"log"
)

func main() {
	//	outputChan := make(chan core.GdbOutputMsg) // ✅ שינוי

	client, outputChan, err := gdb.NewGDBClient()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	client.Start(outputChan)
	client.Send("\n")

	widget := tui.NewGdbModel(
		func(cmd string) { client.Send(cmd) },    // Enter
		func(raw string) { client.SendRaw(raw) }, // arrows
	)

	//	widget := tui.NewGdbModel(func(cmd string) {
	//		if err := client.Send(cmd); err != nil {
	//			log.Println("send error:", err)
	//		}
	//	})
	program := tea.NewProgram(widget)

	// bridge goroutine → Bubble Tea
	go func() {
		for msg := range outputChan {
			program.Send(msg) // ✅ שולח ישירות
		}
	}()

	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

package uitcell

import (
	"github.com/yairgd/promptcore/internal/gdb"

	"github.com/yairgd/promptcore/internal/core"
)

func InitGdbClient(w, h int) (*gdb.GDBClient, *GDBWidget, <-chan core.GdbOutputMsg, error) {

	// --- CHANNEL ---
	outputChan := make(chan core.GdbOutputMsg)

	// --- CLIENT ---
	client, err := gdb.NewGDBClient()
	if err != nil {
		return nil, nil, nil, err
	}

	client.Start(outputChan)
	client.Send("\n")

	// --- WIDGET ---
	widget := NewGDBWidget(client) // ✔️ כאן התיקון

	widget.SetSize(w, h)

	return client, widget, outputChan, nil
}

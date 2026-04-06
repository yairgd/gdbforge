package gdb

import (
	"github.com/yairgd/promptcore/internal/core"
)

func InitGdbClient() (*GDBClient, <-chan core.GdbOutputMsg, error) {
	outputChan := make(chan core.GdbOutputMsg)

	client, err := NewGDBClient()
	if err != nil {
		return nil, nil, err
	}

	client.Start(outputChan)
	client.Send("\n")

	return client, outputChan, nil
}

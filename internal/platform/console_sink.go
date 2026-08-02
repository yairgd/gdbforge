package platform

import (
	"fmt"
)

type ConsoleSink struct {
}

func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{}
}

func (s *ConsoleSink) Write(entry LogEntry) error {
	fmt.Printf(
		"[%s] %-5s %-12s %s\n",
		entry.Time.Format("15:04:05"),
		entry.Level.String(),
		entry.Source,
		entry.Text,
	)

	return nil
}

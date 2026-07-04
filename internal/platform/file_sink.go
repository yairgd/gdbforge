package platform

import (
	"fmt"
	"os"
	"sync"
)

type FileSink struct {
	mu sync.Mutex

	file *os.File
}

func NewFileSink(path string) (*FileSink, error) {

	f, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)

	if err != nil {
		return nil, err
	}

	return &FileSink{
		file: f,
	}, nil
}

func (s *FileSink) Close() error {
	return s.file.Close()
}

func (s *FileSink) Write(entry LogEntry) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := fmt.Fprintf(
		s.file,
		"[%s] %-5s %-12s %s\n",
		entry.Time.Format("2006-01-02 15:04:05"),
		entry.Level.String(),
		entry.Source,
		entry.Text,
	)

	return err
}

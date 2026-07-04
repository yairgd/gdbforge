package platform

import (
	"sync"
	"time"
)

type Level int

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
	Fatal
)

func (l Level) String() string {
	switch l {
	case Trace:
		return "TRACE"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	case Fatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Time   time.Time
	Level  Level
	Source string
	Text   string
}

type Sink interface {
	Write(LogEntry) error
}

type Logger struct {
	mu sync.RWMutex

	level Level

	sinks []Sink

	buffer []LogEntry

	subs []chan LogEntry
}

type NamedLogger struct {
	parent *Logger
	source string
}

func NewLogger() *Logger {
	return &Logger{
		level: Info,
	}
}

func (l *Logger) Named(source string) *NamedLogger {
	return &NamedLogger{
		parent: l,
		source: source,
	}
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.level = level
}

func (l *Logger) AddSink(s Sink) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sinks = append(l.sinks, s)
}

func (l *Logger) Subscribe() <-chan LogEntry {
	ch := make(chan LogEntry, 64)

	l.mu.Lock()
	l.subs = append(l.subs, ch)
	l.mu.Unlock()

	return ch
}

func (l *Logger) Messages() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]LogEntry, len(l.buffer))
	copy(out, l.buffer)

	return out
}

func (l *Logger) Log(level Level, source, text string) {
	l.mu.RLock()
	currentLevel := l.level
	l.mu.RUnlock()

	if level < currentLevel {
		return
	}

	entry := LogEntry{
		Time:   time.Now(),
		Level:  level,
		Source: source,
		Text:   text,
	}

	l.mu.Lock()

	l.buffer = append(l.buffer, entry)

	// Copy slices so we don't hold the lock while writing.
	sinks := append([]Sink(nil), l.sinks...)
	subs := append([]chan LogEntry(nil), l.subs...)

	l.mu.Unlock()

	for _, s := range sinks {
		_ = s.Write(entry)
	}

	for _, sub := range subs {
		select {
		case sub <- entry:
		default:
			// Drop message if subscriber is slow.
		}
	}
}

func (n *NamedLogger) Trace(msg string) {
	n.parent.Log(Trace, n.source, msg)
}

func (n *NamedLogger) Debug(msg string) {
	n.parent.Log(Debug, n.source, msg)
}

func (n *NamedLogger) Info(msg string) {
	n.parent.Log(Info, n.source, msg)
}

func (n *NamedLogger) Warn(msg string) {
	n.parent.Log(Warn, n.source, msg)
}

func (n *NamedLogger) Error(msg string) {
	n.parent.Log(Error, n.source, msg)
}

func (n *NamedLogger) Fatal(msg string) {
	n.parent.Log(Fatal, n.source, msg)
}

package backend

import (
	"context"
	"strings"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdbforge/models"
	"github.com/yairgd/gdbforge/internal/gdbforge/parse"
)

// Kind selects GDB MI vs Delve CLI query shapes.
type Kind int

const (
	GDB Kind = iota
	DLV
)

// Querier runs one debugger console/MI command and returns captured text.
type Querier interface {
	Query(ctx context.Context, cmd string) (string, error)
}

// LogFn reports non-fatal query/parse problems (optional).
type LogFn func(area, msg string)

// ThreadsAndStack queries threads/goroutines and stack frames into models.
func ThreadsAndStack(ctx context.Context, kind Kind, q Querier, log LogFn) (threads []models.ThreadInfo, frames []models.StackFrame, threadsOK, stackOK bool) {
	if q == nil {
		return nil, nil, false, false
	}
	if kind == DLV {
		return threadsAndStackDLV(ctx, q, log)
	}
	return threadsAndStackGDB(ctx, q, log)
}

func threadsAndStackDLV(ctx context.Context, q Querier, log LogFn) (threads []models.ThreadInfo, frames []models.StackFrame, threadsOK, stackOK bool) {
	raw, err := q.Query(ctx, "goroutines")
	if err != nil {
		if log != nil {
			log("threads", err.Error())
		}
	} else if strings.Contains(strings.ToLower(raw), "goroutine") {
		threads = dlv.ParseGoroutines(raw)
		threadsOK = true
		if len(threads) == 0 {
			frames = nil
		}
	}

	raw, err = q.Query(ctx, "stack")
	if err != nil {
		if log != nil {
			log("callstack", err.Error())
		}
	} else {
		frames = dlv.ParseStack(raw)
		if len(frames) > 0 {
			stackOK = true
		}
	}
	return threads, frames, threadsOK, stackOK
}

func threadsAndStackGDB(ctx context.Context, q Querier, log LogFn) (threads []models.ThreadInfo, frames []models.StackFrame, threadsOK, stackOK bool) {
	raw, err := q.Query(ctx, "-thread-info")
	if err != nil {
		if log != nil {
			log("threads", err.Error())
		}
	} else if strings.Contains(raw, "threads=") {
		threads = parse.ParseThreadInfo(raw)
		threadsOK = true
		if len(threads) == 0 {
			frames = nil
		}
	} else if log != nil {
		log("threads", "incomplete -thread-info capture")
	}

	raw, err = q.Query(ctx, "-stack-list-frames")
	if err != nil {
		if log != nil {
			log("callstack", err.Error())
		}
	} else if strings.Contains(raw, "stack=") {
		frames = parse.ParseStackListFrames(raw)
		stackOK = true
	} else if log != nil {
		log("callstack", "incomplete -stack-list-frames capture")
	}
	return threads, frames, threadsOK, stackOK
}

// StackList queries only the backtrace. longCapture uses QueryLong on kgdb serial.
func StackList(ctx context.Context, kind Kind, q Querier, longCapture bool, log LogFn) ([]models.StackFrame, bool) {
	if q == nil {
		return nil, false
	}
	if kind == DLV {
		raw, err := q.Query(ctx, "stack")
		if err != nil {
			if log != nil {
				log("callstack", err.Error())
			}
			return nil, false
		}
		frames := dlv.ParseStack(raw)
		return frames, len(frames) > 0
	}
	query := func(cmd string) (string, error) {
		if longCapture {
			if lq, ok := q.(interface {
				QueryLong(context.Context, string) (string, error)
			}); ok {
				return lq.QueryLong(ctx, cmd)
			}
		}
		return q.Query(ctx, cmd)
	}
	for _, cmd := range []string{"-stack-list-frames 0 50", "-stack-list-frames"} {
		raw, err := query(cmd)
		if err != nil {
			if log != nil {
				log("callstack", err.Error())
			}
			continue
		}
		if !strings.Contains(raw, "stack=") {
			continue
		}
		frames := parse.ParseStackListFrames(raw)
		if len(frames) == 0 {
			continue
		}
		if parse.StackListLooksTruncated(raw, frames) {
			continue
		}
		return frames, true
	}
	return nil, false
}

// MapBreakCmd rewrites break/clear for Delve; GDB cmds pass through.
func MapBreakCmd(kind Kind, cmd string) string {
	if kind == DLV {
		return dlv.MapBreakCmd(cmd)
	}
	return cmd
}

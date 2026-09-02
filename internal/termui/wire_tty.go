package termui

import (
	"strings"
	"sync"
	"time"

	"github.com/yairgd/gdbforge/internal/ptyx"
)

// WireTTYOpts configures WireTTY coalescing and UI refresh.
type WireTTYOpts struct {
	// PostFrame runs on the UI thread after terminal bytes are written (e.g. RequestFrame).
	PostFrame func()
	// OnData runs for each coalesced chunk before it is written to the emulator.
	OnData func(data string)
	// OnExit runs when the PTY session ends (EOF/EIO or subscribe channel closed).
	OnExit func()
	Interval  time.Duration
	MaxBytes  int
}

// WireTTY pumps tty.Subscribe output into ctl.Write with optional coalescing.
func WireTTY(tty *ptyx.TTY, ctl *TerminalController, opts WireTTYOpts) (cancel func()) {
	if tty == nil || ctl == nil {
		return func() {}
	}
	if opts.Interval <= 0 {
		opts.Interval = 50 * time.Millisecond
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 256 * 1024
	}

	ch, unsub := tty.Subscribe()
	done := make(chan struct{})

	var mu sync.Mutex
	var pending strings.Builder

	flush := func(data string, err error) {
		if err != nil {
			return
		}
		if data != "" {
			if opts.OnData != nil {
				opts.OnData(data)
			}
			_ = ctl.Write([]byte(data))
		}
		if opts.PostFrame != nil {
			opts.PostFrame()
		}
	}

	go func() {
		defer func() {
			mu.Lock()
			s := pending.String()
			pending.Reset()
			mu.Unlock()
			flush(s, nil)
		}()

		tick := time.NewTicker(opts.Interval)
		defer tick.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					if opts.OnExit != nil {
						opts.OnExit()
					}
					return
				}
				if msg.Err != nil {
					if ptyx.ClosedError(msg.Err) {
						mu.Lock()
						s := pending.String()
						pending.Reset()
						mu.Unlock()
						flush(s, nil)
						if opts.OnExit != nil {
							opts.OnExit()
						}
						return
					}
					continue
				}
				if msg.Data == "" {
					continue
				}
				mu.Lock()
				pending.WriteString(msg.Data)
				if pending.Len() >= opts.MaxBytes {
					s := pending.String()
					pending.Reset()
					mu.Unlock()
					flush(s, nil)
					continue
				}
				mu.Unlock()
			case <-tick.C:
				mu.Lock()
				if pending.Len() == 0 {
					mu.Unlock()
					continue
				}
				s := pending.String()
				pending.Reset()
				mu.Unlock()
				flush(s, nil)
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		unsub()
	}
}

// WireTTYInput connects keyboard bytes from ctl to tty.SendRaw.
func WireTTYInput(tty *ptyx.TTY, ctl *TerminalController) {
	if ctl == nil {
		return
	}
	if tty == nil {
		ctl.SetInputHandler(nil)
		return
	}
	ctl.SetInputHandler(func(b []byte) error {
		return tty.SendRaw(string(b))
	})
}

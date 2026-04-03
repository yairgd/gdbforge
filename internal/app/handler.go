// app/handler.go
package app

import (
	"github.com/yairgd/promptcore/internal/core"
)

// HandleEvent
func (a App) HandleEvent(e core.Event) ([]core.Event, error) {

	switch ev := e.(type) {

	case core.SubmitMessage:
		return []core.Event{
			core.SubmitMessage{Text: "Echo: " + ev.Text},
		}, nil

	case core.RunCommand:
		if ev.Command == ":q" {
			return []core.Event{
				core.Quit{},
			}, nil
		}
	}

	return nil, nil
}

// app/handler.go
package app

import (
	"github.com/yairgd/promptcore/internal/events"
)

// HandleEvent = הלב של המערכת
func (a App) HandleEvent(e events.Event) ([]events.Event, error) {

	switch ev := e.(type) {

	case events.SubmitMessage:
		return []events.Event{
			events.SubmitMessage{Text: "Echo: " + ev.Text},
		}, nil

	case events.RunCommand:
		if ev.Command == ":q" {
			return []events.Event{
				events.Quit{},
			}, nil
		}
	}

	return nil, nil
}

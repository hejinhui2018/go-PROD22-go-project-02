package audit

import (
	"fleetforge/internal/events"
	"time"
)

type Logger struct {
	Sink interface{ Append(events.Event) error }
	ID   func() string
}

func (l Logger) Record(actor, action, subject string) error {
	return l.Sink.Append(events.Event{ID: l.ID(), Type: "audit." + action, Aggregate: subject, Data: []byte(actor), At: time.Now().UTC()})
}

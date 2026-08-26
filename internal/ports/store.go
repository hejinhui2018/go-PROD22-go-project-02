package ports

import "fleetforge/internal/events"

type EventSink interface{ Append(events.Event) error }

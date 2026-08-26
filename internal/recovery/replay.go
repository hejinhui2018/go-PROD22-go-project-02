package recovery

import "fleetforge/internal/events"

type Applied struct{ IDs map[string]bool }

func NewApplied() *Applied { return &Applied{IDs: map[string]bool{}} }
func (a *Applied) Once(e events.Event) bool {
	if a.IDs[e.ID] {
		return false
	}
	a.IDs[e.ID] = true
	return true
}

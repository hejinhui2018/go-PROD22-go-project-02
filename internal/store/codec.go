package store

import (
	"bufio"
	"encoding/json"
	"fleetforge/internal/domain"
	"fleetforge/internal/events"
	"io"
)

func DecodeEvents(r io.Reader) ([]events.Event, error) {
	sc := bufio.NewScanner(r)
	out := []events.Event{}
	for sc.Scan() {
		var e events.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
func EncodeState(s State) ([]byte, error) { return json.Marshal(s) }
func EmptyState() State {
	return State{Releases: map[string]*domain.Release{}, Devices: map[string]*domain.Device{}, Tasks: map[string]*domain.Task{}, Audit: []events.Event{}, RollbackBatches: map[string]RollbackBatch{}}
}

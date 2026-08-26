package store

import (
	"fleetforge/internal/events"
	"sync"
)

type MemoryStore struct {
	mu     sync.Mutex
	Events []events.Event
	Snap   State
}

func (m *MemoryStore) Append(e events.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Events = append(m.Events, e)
	return nil
}
func (m *MemoryStore) Snapshot(s State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Snap = s
	return nil
}
func (m *MemoryStore) Load() (State, []events.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Snap, m.Events, nil
}

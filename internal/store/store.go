package store

import (
	"encoding/json"
	"fleetforge/internal/domain"
	"fleetforge/internal/events"
	"os"
	"path/filepath"
	"sync"
)

type State struct {
	Releases        map[string]*domain.Release `json:"releases"`
	Devices         map[string]*domain.Device  `json:"devices"`
	Tasks           map[string]*domain.Task    `json:"tasks"`
	Audit           []events.Event             `json:"audit"`
	RollbackBatches map[string]RollbackBatch   `json:"rollback_batches,omitempty"`
}

// RollbackBatch records progress that has crossed the persistence boundary.
// It is part of the recovered state so an interrupted batch can be resumed.
type RollbackBatch struct {
	Reason string `json:"reason,omitempty"`
	Queued int    `json:"queued"`
}
type Store interface {
	Append(events.Event) error
	Snapshot(State) error
	Load() (State, []events.Event, error)
}
type FileStore struct {
	Dir string
	mu  sync.Mutex
}

func NewFileStore(d string) *FileStore { return &FileStore{Dir: d} }
func (f *FileStore) Append(e events.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(f.Dir, 0755); err != nil {
		return err
	}
	b, _ := json.Marshal(e)
	fd, err := os.OpenFile(filepath.Join(f.Dir, "events.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.Write(append(b, '\n'))
	return err
}
func (f *FileStore) Snapshot(s State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(f.Dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.Dir, "snapshot.json"), b, 0644)
}
func (f *FileStore) Load() (State, []events.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := State{Releases: map[string]*domain.Release{}, Devices: map[string]*domain.Device{}, Tasks: map[string]*domain.Task{}, RollbackBatches: map[string]RollbackBatch{}}
	if b, e := os.ReadFile(filepath.Join(f.Dir, "snapshot.json")); e == nil {
		if json.Unmarshal(b, &s) != nil {
			return s, nil, e
		}
	}
	b, e := os.ReadFile(filepath.Join(f.Dir, "events.log"))
	if os.IsNotExist(e) {
		return s, nil, nil
	}
	if e != nil {
		return s, nil, e
	}
	var out []events.Event
	for _, line := range splitLines(b) {
		var ev events.Event
		if json.Unmarshal(line, &ev) == nil {
			out = append(out, ev)
		}
	}
	return s, out, nil
}
func splitLines(b []byte) [][]byte {
	var r [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				r = append(r, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		r = append(r, b[start:])
	}
	return r
}

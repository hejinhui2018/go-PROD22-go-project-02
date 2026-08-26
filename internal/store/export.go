package store

import (
	"encoding/json"
	"errors"
	"fleetforge/internal/events"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Export struct {
	Version     int            `json:"version"`
	GeneratedAt time.Time      `json:"generated_at"`
	State       State          `json:"state"`
	Events      []events.Event `json:"events"`
}

func (d *DurableStore) Export() (Export, error) {
	state, ev, report, err := d.LoadReport()
	if err != nil {
		return Export{}, err
	}
	if report.CorruptRecords > 0 {
		return Export{}, errors.New("cannot export corrupt journal")
	}
	return Export{Version: d.Version, GeneratedAt: time.Now().UTC(), State: state, Events: ev}, nil
}
func (d *DurableStore) ExportTo(path string) error {
	x, err := d.Export()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, b, 0644)
}
func ImportExport(path string) (Export, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Export{}, err
	}
	var x Export
	if err := json.Unmarshal(b, &x); err != nil {
		return Export{}, err
	}
	if x.Version != CurrentVersion {
		return Export{}, errors.New("unsupported export version")
	}
	return x, nil
}

func (x Export) Validate() error {
	if x.Version != CurrentVersion {
		return errors.New("unsupported export version")
	}
	if x.GeneratedAt.IsZero() {
		return errors.New("export timestamp is missing")
	}
	if x.State.Releases == nil || x.State.Devices == nil || x.State.Tasks == nil {
		return errors.New("export state maps are missing")
	}
	return nil
}

func (x Export) EventTypes() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range x.Events {
		if !seen[e.Type] {
			seen[e.Type] = true
			out = append(out, e.Type)
		}
	}
	sort.Strings(out)
	return out
}

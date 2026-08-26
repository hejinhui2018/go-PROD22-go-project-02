package store

import (
	"bufio"
	"encoding/json"
	"fleetforge/internal/events"
	"io"
	"os"
	"time"
)

type JournalStats struct {
	First, Last time.Time
	Types       map[string]int
	Aggregates  map[string]int
}

func ReadJournal(r io.Reader, fn func(events.Event) error) (JournalStats, error) {
	stats := JournalStats{Types: map[string]int{}, Aggregates: map[string]int{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e events.Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if stats.First.IsZero() || e.At.Before(stats.First) {
			stats.First = e.At
		}
		if e.At.After(stats.Last) {
			stats.Last = e.At
		}
		stats.Types[e.Type]++
		stats.Aggregates[e.Aggregate]++
		if fn != nil {
			if err := fn(e); err != nil {
				return stats, err
			}
		}
	}
	return stats, sc.Err()
}
func (d *DurableStore) JournalStats() (JournalStats, error) {
	path, _ := d.paths()
	f, err := os.Open(path)
	if err != nil {
		return JournalStats{Types: map[string]int{}, Aggregates: map[string]int{}}, err
	}
	defer f.Close()
	return ReadJournal(f, nil)
}

package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fleetforge/internal/events"
	"os"
)

type Integrity struct {
	Records, Invalid, Bytes int64
	Version                 int
	Error                   string
}

func (d *DurableStore) Check() Integrity {
	out := Integrity{Version: d.Version}
	path, _ := d.paths()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer f.Close()
	if info, e := f.Stat(); e == nil {
		out.Bytes = info.Size()
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var env Envelope
		if json.Unmarshal(sc.Bytes(), &env) != nil || env.Version != d.Version {
			out.Invalid++
		} else {
			out.Records++
		}
	}
	if err := sc.Err(); err != nil {
		out.Error = err.Error()
	}
	return out
}

func (d *DurableStore) Compact(s State) error {
	if d == nil {
		return errors.New("store is nil")
	}
	return d.Snapshot(s)
}

func (d *DurableStore) EventCount() int { return int(d.Check().Records) }

var _ = events.Event{}

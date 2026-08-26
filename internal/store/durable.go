package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fleetforge/internal/domain"
	"fleetforge/internal/events"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const CurrentVersion = 1

// DurableStore is an append-only JSONL store with crash-safe snapshots.
// The store deliberately keeps the log and snapshot separate: a snapshot is
// a checkpoint, while the log remains the source of truth for recovery.
type DurableStore struct {
	Dir             string
	Version         int
	TruncateCorrupt bool
	mu              sync.Mutex
}

type Envelope struct {
	Version int          `json:"version"`
	Event   events.Event `json:"event"`
}

type RecoveryReport struct {
	EventsRead       int
	CorruptRecords   int
	TruncatedBytes   int64
	SnapshotLoaded   bool
	Version          int
	LastValidOffset  int64
	CorruptionDetail []string
}

func NewDurableStore(dir string) *DurableStore {
	return &DurableStore{Dir: dir, Version: CurrentVersion, TruncateCorrupt: true}
}

func (d *DurableStore) paths() (string, string) {
	return filepath.Join(d.Dir, "events.jsonl"), filepath.Join(d.Dir, "snapshot.json")
}

func (d *DurableStore) ensure() error {
	if strings.TrimSpace(d.Dir) == "" {
		return errors.New("store directory is required")
	}
	return os.MkdirAll(d.Dir, 0755)
}

func (d *DurableStore) Append(e events.Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensure(); err != nil {
		return err
	}
	if e.ID == "" || e.Type == "" || e.At.IsZero() {
		return errors.New("event id, type and timestamp are required")
	}
	path, _ := d.paths()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(Envelope{Version: d.Version, Event: e})
	if err != nil {
		return err
	}
	if _, err = f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return f.Sync()
}

func (d *DurableStore) Snapshot(s State) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensure(); err != nil {
		return err
	}
	_, path := d.paths()
	payload, err := json.Marshal(snapshotEnvelope{Version: d.Version, State: s, WrittenAt: time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	tmp, err := os.CreateTemp(d.Dir, "snapshot-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0644); err == nil {
		_, err = tmp.Write(payload)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return syncDir(d.Dir)
}

type snapshotEnvelope struct {
	Version   int       `json:"version"`
	State     State     `json:"state"`
	WrittenAt time.Time `json:"written_at"`
}

func (d *DurableStore) Load() (State, []events.Event, error) {
	s, ev, _, err := d.LoadReport()
	return s, ev, err
}

func (d *DurableStore) LoadReport() (State, []events.Event, RecoveryReport, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state := EmptyState()
	report := RecoveryReport{Version: d.Version}
	if err := d.ensure(); err != nil {
		return state, nil, report, err
	}
	_, snapshotPath := d.paths()
	if b, err := os.ReadFile(snapshotPath); err == nil {
		var env snapshotEnvelope
		if json.Unmarshal(b, &env) != nil {
			return state, nil, report, errors.New("snapshot is corrupt")
		}
		if env.Version != d.Version {
			return state, nil, report, fmt.Errorf("snapshot version %d is unsupported", env.Version)
		}
		state = env.State
		report.SnapshotLoaded = true
		if state.Releases == nil {
			state.Releases = map[string]*domain.Release{}
		}
		if state.Devices == nil {
			state.Devices = map[string]*domain.Device{}
		}
		if state.Tasks == nil {
			state.Tasks = map[string]*domain.Task{}
		}
		if state.RollbackBatches == nil {
			state.RollbackBatches = map[string]RollbackBatch{}
		}
	}
	logPath, _ := d.paths()
	f, err := os.Open(logPath)
	if os.IsNotExist(err) {
		return state, nil, report, nil
	}
	if err != nil {
		return state, nil, report, err
	}
	defer f.Close()
	var eventsOut []events.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var offset int64
	for scanner.Scan() {
		line := scanner.Bytes()
		lineLen := int64(len(line)) + 1
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil || env.Version != d.Version || env.Event.ID == "" {
			report.CorruptRecords++
			report.CorruptionDetail = append(report.CorruptionDetail, fmt.Sprintf("offset %d: invalid record", offset))
			if d.TruncateCorrupt {
				report.TruncatedBytes = truncateFrom(logPath, offset)
				break
			}
			offset += lineLen
			continue
		}
		eventsOut = append(eventsOut, env.Event)
		report.EventsRead++
		offset += lineLen
		report.LastValidOffset = offset
	}
	if err := scanner.Err(); err != nil {
		return state, eventsOut, report, err
	}
	return state, eventsOut, report, nil
}

func truncateFrom(path string, offset int64) int64 {
	info, err := os.Stat(path)
	if err != nil || offset >= info.Size() {
		return 0
	}
	if err = os.Truncate(path, offset); err != nil {
		return 0
	}
	return info.Size() - offset
}

func syncDir(dir string) error {
	// Windows does not support fsync on directory handles; the file itself was
	// already synced before the atomic replacement.
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Compile-time checks keep the durable implementation interchangeable.
var _ Store = (*DurableStore)(nil)

// io import is retained for stream helpers shared by file recovery tools.
var _ io.Reader

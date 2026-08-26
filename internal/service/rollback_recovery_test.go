package service

import (
	"errors"
	"fleetforge/internal/domain"
	"fleetforge/internal/events"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"testing"
	"time"
)

type rollbackRecoveryStore struct {
	snapshot        store.State
	events          []events.Event
	replayFrom      int
	committedEvents int
	snapshotCount   int
	failAtSnapshot  int
	failed          bool
}

func (f *rollbackRecoveryStore) Append(event events.Event) error {
	f.events = append(f.events, cloneEvent(event))
	return nil
}

func (f *rollbackRecoveryStore) Snapshot(state store.State) error {
	f.snapshotCount++
	if f.snapshotCount == f.failAtSnapshot && !f.failed {
		f.failed = true
		return errors.New("replace snapshot: Access is denied")
	}
	f.snapshot = cloneState(state)
	f.replayFrom = len(f.events)
	f.committedEvents = len(f.events)
	return nil
}

func (f *rollbackRecoveryStore) Load() (store.State, []events.Event, error) {
	start, committed := f.replayFrom, f.committedEvents
	if start > len(f.events) {
		start = len(f.events)
	}
	if committed > len(f.events) {
		committed = len(f.events)
	}
	if committed < start {
		committed = start
	}
	result := make([]events.Event, committed-start)
	for i := range result {
		result[i] = cloneEvent(f.events[start+i])
	}
	return cloneState(f.snapshot), result, nil
}

func cloneEvent(event events.Event) events.Event {
	event.Data = append([]byte(nil), event.Data...)
	return event
}

func cloneState(state store.State) store.State {
	return store.State{
		Releases:        cloneReleases(state.Releases),
		Devices:         cloneDevices(state.Devices),
		Tasks:           cloneTasks(state.Tasks),
		Audit:           append([]events.Event(nil), state.Audit...),
		RollbackBatches: cloneRollbackBatches(state.RollbackBatches),
	}
}

func cloneReleases(source map[string]*domain.Release) map[string]*domain.Release {
	result := map[string]*domain.Release{}
	for id, release := range source {
		copy := *release
		copy.Devices = append([]string(nil), release.Devices...)
		result[id] = &copy
	}
	return result
}

func cloneDevices(source map[string]*domain.Device) map[string]*domain.Device {
	result := map[string]*domain.Device{}
	for id, device := range source {
		copy := *device
		copy.Capabilities = append([]string(nil), device.Capabilities...)
		result[id] = &copy
	}
	return result
}

func cloneTasks(source map[string]*domain.Task) map[string]*domain.Task {
	result := map[string]*domain.Task{}
	for id, task := range source {
		copy := *task
		result[id] = &copy
	}
	return result
}

func cloneRollbackBatches(source map[string]store.RollbackBatch) map[string]store.RollbackBatch {
	result := map[string]store.RollbackBatch{}
	for id, batch := range source {
		result[id] = batch
	}
	return result
}

func TestRollbackQueueResumesAfterInterruptedPersistence(t *testing.T) {
	clock := fixedRollbackClock{now: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}
	persisted := &rollbackRecoveryStore{}
	first := New(persisted, clock, &ports.SequenceID{})
	release, err := first.CreateRelease("2.0.0", []string{"dev-a", "dev-b", "dev-c"}, 1, 1, 1, domain.RollbackPolicy{})
	if err != nil {
		t.Fatalf("create release: %v", err)
	}
	for _, task := range first.State.Tasks {
		task.Status = domain.TaskCompleted
		task.UpdatedAt = clock.Now()
	}
	first.State.Releases[release.ID].Status = domain.StatusRollbackPending
	persisted.Snapshot(first.State)
	persisted.failAtSnapshot = persisted.snapshotCount + 2

	if _, err := first.QueueRollback(release.ID, "bad firmware"); err == nil {
		t.Fatal("expected the first rollback attempt to fail at the snapshot boundary")
	}

	afterRestart := New(persisted, clock, &ports.SequenceID{})
	queued, err := afterRestart.QueueRollback(release.ID, "bad firmware")
	if err != nil {
		t.Fatalf("resume rollback queue: %v", err)
	}
	if len(queued) != 3 {
		t.Fatalf("resume returned %d tasks, want 3", len(queued))
	}
	for _, device := range []string{"dev-a", "dev-b", "dev-c"} {
		count := 0
		for _, task := range afterRestart.State.Tasks {
			if task.ReleaseID == release.ID && task.DeviceID == device {
				if task.Status != domain.TaskRollbackQueued {
					t.Fatalf("%s status = %s, want rollback-queued", device, task.Status)
				}
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%s task count = %d, want 1", device, count)
		}
	}

	for _, device := range []string{"dev-a", "dev-b", "dev-c"} {
		task, err := afterRestart.ClaimRollback(device, "rollback-agent")
		if err != nil {
			t.Fatalf("claim %s: %v", device, err)
		}
		if err := afterRestart.CompleteRollback(task.ID, "rollback-agent"); err != nil {
			t.Fatalf("complete %s: %v", device, err)
		}
	}
	if got := afterRestart.State.Releases[release.ID].Status; got != domain.StatusRolledBack {
		t.Fatalf("release status = %s, want rolled-back", got)
	}
}

type fixedRollbackClock struct{ now time.Time }

func (c fixedRollbackClock) Now() time.Time { return c.now }

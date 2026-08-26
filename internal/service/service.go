package service

import (
	"encoding/json"
	"errors"
	"fleetforge/internal/domain"
	"fleetforge/internal/events"
	"fleetforge/internal/ports"
	"fleetforge/internal/store"
	"time"
)

type Service struct {
	St    store.Store
	Clock ports.Clock
	IDs   ports.IDGenerator
	State store.State
}

func New(st store.Store, c ports.Clock, ids ports.IDGenerator) *Service {
	s := &Service{St: st, Clock: c, IDs: ids, State: store.State{Releases: map[string]*domain.Release{}, Devices: map[string]*domain.Device{}, Tasks: map[string]*domain.Task{}}}
	s.Recover()
	return s
}
func (s *Service) Recover() {
	st, ev, _ := s.St.Load()
	s.State = st
	if len(ev) > 0 {
		// Rebuild the audit index from the replay stream to avoid duplicating
		// entries already captured in a checkpoint.
		s.State.Audit = nil
	}
	if s.State.Releases == nil {
		s.State.Releases = map[string]*domain.Release{}
	}
	if s.State.Devices == nil {
		s.State.Devices = map[string]*domain.Device{}
	}
	if s.State.Tasks == nil {
		s.State.Tasks = map[string]*domain.Task{}
	}
	for _, e := range ev {
		s.apply(e)
	}
}
func (s *Service) emit(t, a string, v any) error {
	b, _ := json.Marshal(v)
	e := events.Event{ID: s.IDs.New("evt"), Type: t, Aggregate: a, Data: b, At: s.Clock.Now()}
	if err := s.St.Append(e); err != nil {
		return err
	}
	s.apply(e)
	return s.St.Snapshot(s.State)
}
func (s *Service) apply(e events.Event) {
	if s.State.Audit == nil {
		s.State.Audit = []events.Event{}
	}
	if len(s.State.Audit) == 0 || s.State.Audit[len(s.State.Audit)-1].ID != e.ID {
		s.State.Audit = append(s.State.Audit, e)
	}
	switch e.Type {
	case "release.created":
		var r domain.Release
		if json.Unmarshal(e.Data, &r) == nil {
			s.State.Releases[r.ID] = &r
		}
	case "device.registered":
		var d domain.Device
		if json.Unmarshal(e.Data, &d) == nil {
			s.State.Devices[d.ID] = &d
		}
	case "task.created":
		var t domain.Task
		if json.Unmarshal(e.Data, &t) == nil {
			s.State.Tasks[t.ID] = &t
		}
	case "task.updated":
		var t domain.Task
		if json.Unmarshal(e.Data, &t) == nil {
			s.State.Tasks[t.ID] = &t
		}
	case "release.updated":
		var r domain.Release
		if json.Unmarshal(e.Data, &r) == nil {
			s.State.Releases[r.ID] = &r
		}
	}
}
func (s *Service) CreateRelease(v string, dev []string, batch, conc, retries int, rb domain.RollbackPolicy) (*domain.Release, error) {
	if v == "" || len(dev) == 0 || batch < 1 || conc < 1 || retries < 0 {
		return nil, errors.New("invalid release parameters")
	}
	r := &domain.Release{ID: s.IDs.New("rel"), Version: v, Devices: dev, BatchSize: batch, MaxConcurrent: conc, RetryLimit: retries, Rollback: rb, Status: domain.StatusQueued, CreatedAt: s.Clock.Now(), UpdatedAt: s.Clock.Now()}
	if err := s.emit("release.created", r.ID, r); err != nil {
		return nil, err
	}
	for _, d := range dev {
		t := &domain.Task{ID: s.IDs.New("task"), ReleaseID: r.ID, DeviceID: d, Status: domain.TaskQueued, UpdatedAt: s.Clock.Now()}
		if err := s.emit("task.created", t.ID, t); err != nil {
			return nil, err
		}
	}
	return r, nil
}
func (s *Service) RegisterDevice(id, fw string, caps []string) (*domain.Device, error) {
	if id == "" {
		return nil, errors.New("device id required")
	}
	d := &domain.Device{ID: id, Firmware: fw, Capabilities: caps, RegisteredAt: s.Clock.Now(), LastSeen: s.Clock.Now(), Online: true}
	return d, s.emit("device.registered", id, d)
}
func (s *Service) Claim(device, agent string) (*domain.Task, error) {
	return s.ClaimWithTTL(device, agent, 5*time.Minute)
}
func (s *Service) UpdateTask(id, action, reason string) (*domain.Task, error) {
	t, ok := s.State.Tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	now := s.Clock.Now()
	switch action {
	case "ack":
		if t.Status != domain.TaskLeased {
			return nil, errors.New("task not leased")
		}
		t.Status = domain.TaskInstalling
	case "reject":
		t.Status = domain.TaskRejected
		t.Error = reason
		t.NextRetry = now.Add(time.Duration(t.Attempts+1) * time.Minute)
	case "complete":
		if t.Status != domain.TaskInstalling && t.Status != domain.TaskAwaiting {
			return nil, errors.New("task not active")
		}
		t.Status = domain.TaskCompleted
		t.LeaseOwner = ""
		t.LeaseUntil = time.Time{}
	default:
		return nil, errors.New("unknown task action")
	}
	t.UpdatedAt = now
	s.emit("task.updated", t.ID, t)
	s.reconcile(t.ReleaseID)
	return t, nil
}
func (s *Service) reconcile(id string) {
	r := s.State.Releases[id]
	if r == nil {
		return
	}
	done, fail := 0, 0
	for _, t := range s.State.Tasks {
		if t.ReleaseID == id {
			if t.Status == domain.TaskCompleted {
				done++
			}
			if t.Status == domain.TaskFailed {
				fail++
			}
		}
	}
	r.Completed = done
	r.Failed = fail
	r.UpdatedAt = s.Clock.Now()
	if done == len(r.Devices) {
		r.Status = domain.StatusCompleted
	}
	if fail > r.Rollback.MaxFailures && r.Rollback.Auto {
		r.Status = domain.StatusRollbackPending
	}
	s.emit("release.updated", r.ID, r)
}
func (s *Service) SetRelease(id, action string) error {
	r, ok := s.State.Releases[id]
	if !ok {
		return errors.New("release not found")
	}
	if action == "pause" {
		if !r.CanPause() {
			return errors.New("release cannot pause")
		}
		r.Status = domain.StatusPaused
		r.PausedAt = s.Clock.Now()
		if r.PauseWindow > 0 {
			r.ResumeNotBefore = r.PausedAt.Add(r.PauseWindow)
		} else {
			r.ResumeNotBefore = time.Time{}
		}
		r.DispatchNotBefore = r.ResumeNotBefore
	} else if action == "resume" {
		if r.Status != domain.StatusPaused {
			return errors.New("release not paused")
		}
		r.Status = domain.StatusQueued
		// An early resume changes the release status immediately, but the
		// configured pause window still gates new leases.
		if !r.ResumeNotBefore.IsZero() && !s.Clock.Now().Before(r.ResumeNotBefore) {
			r.ResumeNotBefore = time.Time{}
			r.DispatchNotBefore = time.Time{}
		}
	} else if action == "rollback" {
		r.Status = domain.StatusRollbackPending
	} else {
		return errors.New("unknown release action")
	}
	r.UpdatedAt = s.Clock.Now()
	return s.emit("release.updated", id, r)
}
func (s *Service) ReapExpired() {
	s.ReapExpiredDetailed()
}

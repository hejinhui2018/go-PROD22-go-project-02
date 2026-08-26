package service

import (
	"errors"
	"fleetforge/internal/domain"
	"time"
)

// ReportPreflight records an agent's compatibility check.
func (s *Service) ReportPreflight(taskID string, ok bool, detail string) error {
	t, found := s.State.Tasks[taskID]
	if !found {
		return ErrNotFound
	}
	if t.Status != domain.TaskInstalling && t.Status != domain.TaskLeased {
		return ErrConflict
	}
	if ok {
		t.Status = domain.TaskAwaiting
		t.Error = ""
	} else {
		t.Status = domain.TaskFailed
		t.Error = detail
	}
	t.UpdatedAt = s.Clock.Now()
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return err
	}
	s.reconcile(t.ReleaseID)
	return nil
}

// ConfirmInstall is idempotent for completed tasks and rejects stale leases.
func (s *Service) ConfirmInstall(taskID, agent string) error {
	t, found := s.State.Tasks[taskID]
	if !found {
		return ErrNotFound
	}
	if t.Status == domain.TaskCompleted {
		return nil
	}
	if t.LeaseOwner != agent || s.Clock.Now().After(t.LeaseUntil) {
		return errors.New("lease is not owned by agent")
	}
	if t.Status != domain.TaskAwaiting {
		return ErrConflict
	}
	t.Status = domain.TaskCompleted
	t.LeaseOwner = ""
	t.LeaseUntil = time.Time{}
	t.UpdatedAt = s.Clock.Now()
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return err
	}
	s.reconcile(t.ReleaseID)
	return nil
}

func (s *Service) DeviceHeartbeat(id string, online bool) error {
	d, ok := s.State.Devices[id]
	if !ok {
		return ErrNotFound
	}
	d.Online = online
	d.LastSeen = s.Clock.Now()
	return s.emit("device.registered", id, d)
}

func (s *Service) ReleaseProgress(id string) (completed, total, failed int, err error) {
	r, ok := s.State.Releases[id]
	if !ok {
		return 0, 0, 0, ErrNotFound
	}
	for _, t := range s.State.Tasks {
		if t.ReleaseID != id {
			continue
		}
		total++
		if t.Status == domain.TaskCompleted {
			completed++
		}
		if t.Status == domain.TaskFailed {
			failed++
		}
	}
	if total != len(r.Devices) {
		err = errors.New("task index inconsistent")
	}
	return
}

func (s *Service) RetryTask(id string) error {
	t, ok := s.State.Tasks[id]
	if !ok {
		return ErrNotFound
	}
	if t.Status != domain.TaskFailed && t.Status != domain.TaskRejected {
		return ErrConflict
	}
	r := s.State.Releases[t.ReleaseID]
	if t.Attempts > r.RetryLimit {
		return errors.New("retry limit reached")
	}
	t.Status = domain.TaskQueued
	t.NextRetry = s.Clock.Now()
	t.Error = ""
	t.UpdatedAt = s.Clock.Now()
	return s.emit("task.updated", id, t)
}

func (s *Service) TasksByDevice(device string) []*domain.Task {
	out := []*domain.Task{}
	for _, t := range s.State.Tasks {
		if t.DeviceID == device {
			out = append(out, t)
		}
	}
	return out
}
func (s *Service) OnlineDevices() []*domain.Device {
	out := []*domain.Device{}
	for _, d := range s.State.Devices {
		if d.Online {
			out = append(out, d)
		}
	}
	return out
}

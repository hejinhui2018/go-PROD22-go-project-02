package service

import (
	"errors"
	"fleetforge/internal/audit"
	"fleetforge/internal/domain"
	"fleetforge/internal/store"
	"sort"
	"strings"
	"time"
)

type RecoveryInfo struct {
	Loaded    bool
	Events    int
	Corrupt   int
	Truncated int64
	LastError string
}

func (s *Service) RecoveryInfo() RecoveryInfo {
	if d, ok := s.St.(*store.DurableStore); ok {
		_, _, r, err := d.LoadReport()
		out := RecoveryInfo{Loaded: r.SnapshotLoaded, Events: r.EventsRead, Corrupt: r.CorruptRecords, Truncated: r.TruncatedBytes}
		if err != nil {
			out.LastError = err.Error()
		}
		return out
	}
	return RecoveryInfo{Loaded: true}
}

func (s *Service) AuditSummary() any {
	return audit.Build(s.State.Audit, time.Time{}, time.Time{}, 10)
}

func (s *Service) ListReleases() []*domain.Release {
	out := make([]*domain.Release, 0, len(s.State.Releases))
	for _, r := range s.State.Releases {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Service) StartTask(id, agent string) error {
	t, ok := s.State.Tasks[id]
	if !ok {
		return ErrNotFound
	}
	if err := t.VerifyLease(agent, s.Clock.Now()); err != nil {
		return err
	}
	if err := t.Transition(domain.TaskInstalling); err != nil {
		return err
	}
	t.UpdatedAt = s.Clock.Now()
	return s.emit("task.updated", id, t)
}

func (s *Service) RejectTask(id, agent, reason string) error {
	t, ok := s.State.Tasks[id]
	if !ok {
		return ErrNotFound
	}
	if err := t.VerifyLease(agent, s.Clock.Now()); err != nil {
		return err
	}
	r := s.State.Releases[t.ReleaseID]
	if r == nil {
		return errors.New("release missing")
	}
	t.ScheduleRetry(s.Clock.Now(), time.Second, r.RetryLimit, strings.TrimSpace(reason))
	return s.emit("task.updated", id, t)
}

func (s *Service) ReportProgress(id, agent string, percent int, detail string) error {
	if percent < 0 || percent > 100 {
		return errors.New("progress must be between 0 and 100")
	}
	t, ok := s.State.Tasks[id]
	if !ok {
		return ErrNotFound
	}
	if err := t.VerifyLease(agent, s.Clock.Now()); err != nil {
		return err
	}
	if t.Status != domain.TaskInstalling && t.Status != domain.TaskAwaiting {
		return ErrConflict
	}
	if detail != "" {
		t.Error = detail
	}
	if percent == 100 {
		t.Status = domain.TaskAwaiting
	}
	t.UpdatedAt = s.Clock.Now()
	return s.emit("task.updated", id, t)
}

func (s *Service) RequestRollback(id, reason string) error {
	r, ok := s.State.Releases[id]
	if !ok {
		return ErrNotFound
	}
	if r.Terminal() && r.Status != domain.StatusCompleted {
		return ErrConflict
	}
	if err := r.Transition(domain.StatusRollbackPending); err != nil {
		return err
	}
	r.UpdatedAt = s.Clock.Now()
	r.Failed++
	return s.emit("release.updated", id, r)
}

func (s *Service) Confirm(id, agent string) error { return s.ConfirmInstall(id, agent) }

func (s *Service) Snapshot() error { return s.St.Snapshot(s.State) }

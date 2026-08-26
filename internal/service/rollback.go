package service

import (
	"errors"
	"fleetforge/internal/domain"
	"fleetforge/internal/store"
	"sort"
	"time"
)

type ReapResult struct {
	Requeued, Failed int
	Releases         []string
}

func (s *Service) RenewLease(taskID, agent string, ttl time.Duration) (*domain.Task, error) {
	if ttl <= 0 || ttl > time.Hour {
		return nil, errors.New("lease extension must be between zero and one hour")
	}
	t, ok := s.State.Tasks[taskID]
	if !ok {
		return nil, ErrNotFound
	}
	now := s.Clock.Now()
	if err := t.VerifyLease(agent, now); err != nil {
		return nil, err
	}
	if t.Status != domain.TaskLeased && t.Status != domain.TaskInstalling && t.Status != domain.TaskAwaiting && t.Status != domain.TaskRollingBack {
		return nil, ErrConflict
	}
	t.LeaseUntil = now.Add(ttl)
	t.UpdatedAt = now
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) FailTask(taskID, agent, reason string, retryable bool) (*domain.Task, error) {
	t, ok := s.State.Tasks[taskID]
	if !ok {
		return nil, ErrNotFound
	}
	if t.Status == domain.TaskFailed && t.Error == reason {
		return t, nil
	}
	if err := t.VerifyLease(agent, s.Clock.Now()); err != nil {
		return nil, err
	}
	r := s.State.Releases[t.ReleaseID]
	if r == nil {
		return nil, errors.New("task release is missing")
	}
	if retryable {
		t.ScheduleRetry(s.Clock.Now(), time.Second, r.RetryLimit, reason)
	} else {
		t.Status = domain.TaskFailed
		t.Error = reason
		t.ReleaseLease()
		t.UpdatedAt = s.Clock.Now()
	}
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return nil, err
	}
	s.reconcile(t.ReleaseID)
	return t, nil
}

func (s *Service) QueueRollback(releaseID, reason string) ([]*domain.Task, error) {
	r, ok := s.State.Releases[releaseID]
	if !ok {
		return nil, ErrNotFound
	}
	if s.State.RollbackBatches == nil {
		s.State.RollbackBatches = map[string]store.RollbackBatch{}
	}
	// The checkpoint records persisted progress, not batch completion. A
	// snapshot failure can leave only a prefix queued, so resume missing tasks.
	if !r.Terminal() && r.Status != domain.StatusRollbackPending {
		if err := r.Transition(domain.StatusRollbackPending); err != nil {
			return nil, err
		}
	}
	queued := []*domain.Task{}
	candidates := []*domain.Task{}
	for _, t := range s.State.Tasks {
		if t.ReleaseID != releaseID {
			continue
		}
		switch t.Status {
		case domain.TaskRollbackQueued, domain.TaskRollingBack, domain.TaskRolledBack:
			queued = append(queued, t)
		case domain.TaskCompleted, domain.TaskFailed:
			candidates = append(candidates, t)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].DeviceID < candidates[j].DeviceID })
	for _, t := range candidates {
		if err := t.Transition(domain.TaskRollbackQueued); err != nil {
			return nil, err
		}
		t.Error = reason
		t.ReleaseLease()
		t.UpdatedAt = s.Clock.Now()
		if err := s.emit("task.updated", t.ID, t); err != nil {
			return nil, err
		}
		queued = append(queued, t)
	}
	r.UpdatedAt = s.Clock.Now()
	if err := s.emit("release.updated", r.ID, r); err != nil {
		return nil, err
	}
	sort.Slice(queued, func(i, j int) bool { return queued[i].DeviceID < queued[j].DeviceID })
	return queued, nil
}

func (s *Service) ClaimRollback(device, agent string) (*domain.Task, error) {
	now := s.Clock.Now()
	candidates := []*domain.Task{}
	for _, t := range s.State.Tasks {
		if t.DeviceID == device && t.Status == domain.TaskRollbackQueued {
			candidates = append(candidates, t)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].UpdatedAt.Before(candidates[j].UpdatedAt) })
	if len(candidates) == 0 {
		return nil, ErrNoTask
	}
	t := candidates[0]
	t.Status = domain.TaskRollingBack
	t.LeaseOwner = agent
	t.LeaseUntil = now.Add(5 * time.Minute)
	t.Attempts++
	t.UpdatedAt = now
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) CompleteRollback(taskID, agent string) error {
	t, ok := s.State.Tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	if t.Status == domain.TaskRolledBack {
		return nil
	}
	if err := t.VerifyLease(agent, s.Clock.Now()); err != nil {
		return err
	}
	if err := t.Transition(domain.TaskRolledBack); err != nil {
		return err
	}
	t.ReleaseLease()
	t.UpdatedAt = s.Clock.Now()
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return err
	}
	all := true
	r := s.State.Releases[t.ReleaseID]
	for _, other := range s.State.Tasks {
		if other.ReleaseID == t.ReleaseID && other.Status != domain.TaskRolledBack && other.Status != domain.TaskQueued && other.Status != domain.TaskRejected {
			all = false
			break
		}
	}
	if all {
		if err := r.Transition(domain.StatusRolledBack); err != nil {
			return err
		}
		r.UpdatedAt = s.Clock.Now()
		return s.emit("release.updated", r.ID, r)
	}
	return nil
}

func (s *Service) ReapExpiredDetailed() ReapResult {
	now := s.Clock.Now()
	out := ReapResult{}
	changed := map[string]bool{}
	for _, t := range s.State.Tasks {
		active := t.Status == domain.TaskLeased || t.Status == domain.TaskInstalling || t.Status == domain.TaskAwaiting || t.Status == domain.TaskRollingBack
		if !active || t.LeaseUntil.IsZero() || !now.After(t.LeaseUntil) {
			continue
		}
		r := s.State.Releases[t.ReleaseID]
		if r == nil {
			continue
		}
		if t.Status == domain.TaskRollingBack {
			t.Status = domain.TaskRollbackQueued
			out.Requeued++
		} else if t.Attempts > r.RetryLimit {
			t.Status = domain.TaskFailed
			out.Failed++
		} else {
			t.Status = domain.TaskQueued
			t.NextRetry = now.Add(time.Duration(t.Attempts) * time.Minute)
			out.Requeued++
		}
		t.ReleaseLease()
		t.Error = "lease expired"
		t.UpdatedAt = now
		_ = s.emit("task.updated", t.ID, t)
		changed[t.ReleaseID] = true
	}
	for id := range changed {
		s.reconcile(id)
		out.Releases = append(out.Releases, id)
	}
	sort.Strings(out.Releases)
	return out
}

package service

import (
	"errors"
	"fleetforge/internal/domain"
	"sort"
	"time"
)

type DispatchDecision struct {
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
	Capacity int    `json:"capacity"`
	Active   int    `json:"active"`
}

func (s *Service) DispatchDecision(releaseID string) DispatchDecision {
	r := s.State.Releases[releaseID]
	if r == nil {
		return DispatchDecision{Reason: "release not found"}
	}
	tasks := s.ListTasks(releaseID)
	capacity := r.Capacity(tasks)
	active := 0
	for _, t := range tasks {
		if t.Status == domain.TaskLeased || t.Status == domain.TaskInstalling || t.Status == domain.TaskAwaiting {
			active++
		}
	}
	if r.Terminal() {
		return DispatchDecision{Reason: "release is terminal", Capacity: capacity, Active: active}
	}
	if r.Status == domain.StatusPaused {
		return DispatchDecision{Reason: "release is paused", Capacity: capacity, Active: active}
	}
	if !r.DispatchOpen(s.Clock.Now()) {
		return DispatchDecision{Reason: "pause window is still active", Capacity: capacity, Active: active}
	}
	if r.Status == domain.StatusRollbackPending {
		return DispatchDecision{Reason: "release is rolling back", Capacity: capacity, Active: active}
	}
	if capacity <= 0 {
		return DispatchDecision{Reason: "concurrency limit reached", Capacity: 0, Active: active}
	}
	return DispatchDecision{Allowed: true, Capacity: capacity, Active: active}
}

func (s *Service) EligibleTasks(device string, now time.Time) []*domain.Task {
	out := []*domain.Task{}
	for _, t := range s.State.Tasks {
		if t.DeviceID != device || (t.Status != domain.TaskQueued && t.Status != domain.TaskRejected) {
			continue
		}
		if !t.NextRetry.IsZero() && now.Before(t.NextRetry) {
			continue
		}
		if !s.DispatchDecision(t.ReleaseID).Allowed {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := s.State.Releases[out[i].ReleaseID], s.State.Releases[out[j].ReleaseID]
		if ri.CreatedAt.Equal(rj.CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return ri.CreatedAt.Before(rj.CreatedAt)
	})
	return out
}

func (s *Service) ClaimWithTTL(device, agent string, ttl time.Duration) (*domain.Task, error) {
	if device == "" || agent == "" {
		return nil, errors.New("device and agent are required")
	}
	if ttl <= 0 || ttl > time.Hour {
		return nil, errors.New("invalid lease duration")
	}
	d, ok := s.State.Devices[device]
	if !ok {
		return nil, ErrNotFound
	}
	if !d.Online {
		return nil, errors.New("device is offline")
	}
	items := s.EligibleTasks(device, s.Clock.Now())
	if len(items) == 0 {
		return nil, ErrNoTask
	}
	t := items[0]
	r := s.State.Releases[t.ReleaseID]
	if err := t.Lease(agent, s.Clock.Now(), ttl); err != nil {
		return nil, err
	}
	if err := s.emit("task.updated", t.ID, t); err != nil {
		return nil, err
	}
	if r.Status == domain.StatusQueued {
		if err := r.Transition(domain.StatusPreflight); err != nil {
			return nil, err
		}
		r.UpdatedAt = s.Clock.Now()
		if err := s.emit("release.updated", r.ID, r); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (s *Service) ReleasePlan(id string) (map[string]any, error) {
	r, ok := s.State.Releases[id]
	if !ok {
		return nil, ErrNotFound
	}
	tasks := s.ListTasks(id)
	byStatus := map[domain.TaskStatus]int{}
	for _, t := range tasks {
		byStatus[t.Status]++
	}
	decision := s.DispatchDecision(id)
	return map[string]any{"release": r, "tasks": len(tasks), "by_status": byStatus, "dispatch": decision, "rollback_required": r.ShouldRollback()}, nil
}

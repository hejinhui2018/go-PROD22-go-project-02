package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidTaskTransition    = errors.New("invalid task state transition")
	ErrInvalidReleaseTransition = errors.New("invalid release state transition")
	ErrLeaseExpired             = errors.New("task lease expired")
	ErrLeaseOwner               = errors.New("task lease owned by another agent")
	ErrReleasePaused            = errors.New("release is paused")
)

func (t *Task) Transition(to TaskStatus) error {
	if t.Status == to {
		return nil
	}
	if !ValidTransition(t.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTaskTransition, t.Status, to)
	}
	t.Status = to
	return nil
}

func (t *Task) Lease(agent string, now time.Time, ttl time.Duration) error {
	if agent == "" {
		return errors.New("agent is required")
	}
	if t.Status != TaskQueued && t.Status != TaskRejected {
		return fmt.Errorf("%w: %s cannot be leased", ErrInvalidTaskTransition, t.Status)
	}
	if !t.NextRetry.IsZero() && now.Before(t.NextRetry) {
		return errors.New("retry backoff is active")
	}
	if err := t.Transition(TaskLeased); err != nil {
		return err
	}
	t.LeaseOwner, t.LeaseUntil, t.Attempts, t.UpdatedAt = agent, now.Add(ttl), t.Attempts+1, now
	return nil
}

func (t *Task) VerifyLease(agent string, now time.Time) error {
	if t.LeaseOwner != agent {
		return ErrLeaseOwner
	}
	if !t.LeaseUntil.IsZero() && now.After(t.LeaseUntil) {
		return ErrLeaseExpired
	}
	return nil
}

func (t *Task) ReleaseLease() { t.LeaseOwner = ""; t.LeaseUntil = time.Time{} }

func (t *Task) ScheduleRetry(now time.Time, base time.Duration, limit int, reason string) bool {
	t.Error = reason
	t.ReleaseLease()
	t.UpdatedAt = now
	if t.Attempts > limit {
		t.Status = TaskFailed
		t.NextRetry = time.Time{}
		return false
	}
	t.Status = TaskRejected
	delay := base * time.Duration(1<<min(t.Attempts-1, 8))
	t.NextRetry = now.Add(delay)
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ValidReleaseTransition(from, to ReleaseStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusQueued:
		return to == StatusPreflight || to == StatusPaused || to == StatusRollbackPending || to == StatusFailed
	case StatusPreflight:
		return to == StatusInstalling || to == StatusPaused || to == StatusRollbackPending || to == StatusFailed
	case StatusInstalling:
		return to == StatusAwaiting || to == StatusPaused || to == StatusRollbackPending || to == StatusFailed || to == StatusCompleted
	case StatusAwaiting:
		return to == StatusInstalling || to == StatusPaused || to == StatusRollbackPending || to == StatusFailed || to == StatusCompleted
	case StatusPaused:
		return to == StatusQueued || to == StatusRollbackPending || to == StatusFailed
	case StatusRollbackPending:
		return to == StatusRolledBack || to == StatusFailed
	}
	return false
}

func (r *Release) Transition(to ReleaseStatus) error {
	if !ValidReleaseTransition(r.Status, to) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidReleaseTransition, r.Status, to)
	}
	r.Status = to
	return nil
}

func (r *Release) Active(tasks []*Task) int { return r.MaxConcurrent - r.Capacity(tasks) }

func (r *Release) Ready(tasks []*Task, now time.Time) bool {
	if r.Terminal() || r.Status == StatusPaused {
		return false
	}
	_ = now
	return r.Capacity(tasks) > 0
}

func (r *Release) Recount(tasks []*Task) (completed, failed, pending int) {
	for _, t := range tasks {
		if t.ReleaseID != r.ID {
			continue
		}
		switch t.Status {
		case TaskCompleted:
			completed++
		case TaskFailed:
			failed++
		default:
			pending++
		}
	}
	return
}

func (r *Release) Progress(tasks []*Task) float64 {
	completed, _, _ := r.Recount(tasks)
	if len(r.Devices) == 0 {
		return 0
	}
	return float64(completed) / float64(len(r.Devices))
}

func (r *Release) FailureRate(tasks []*Task) float64 {
	_, failed, _ := r.Recount(tasks)
	if len(r.Devices) == 0 {
		return 0
	}
	return float64(failed) / float64(len(r.Devices))
}

func (t *Task) LeaseActive(now time.Time) bool {
	return !t.LeaseUntil.IsZero() && now.Before(t.LeaseUntil)
}

func (t *Task) RetryReady(now time.Time) bool {
	return t.NextRetry.IsZero() || !now.Before(t.NextRetry)
}

func (t *Task) IsActive() bool {
	return t.Status == TaskLeased || t.Status == TaskInstalling || t.Status == TaskAwaiting || t.Status == TaskRollingBack
}

func (t *Task) IsTerminal() bool {
	return t.Status == TaskCompleted || t.Status == TaskFailed || t.Status == TaskRolledBack
}

func (r *Release) Remaining(tasks []*Task) int {
	completed, _, _ := r.Recount(tasks)
	remaining := len(r.Devices) - completed
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (r *Release) HasCapacity(tasks []*Task) bool { return r.Capacity(tasks) > 0 }

func (r *Release) IsPaused() bool { return r.Status == StatusPaused }

func (r *Release) IsRollbackPending() bool { return r.Status == StatusRollbackPending }

func (r *Release) IsQueued() bool { return r.Status == StatusQueued }

func (r *Release) DispatchOpen(now time.Time) bool {
	if r.Status == StatusPaused || r.Terminal() {
		return false
	}
	return r.DispatchNotBefore.IsZero() || !now.Before(r.DispatchNotBefore)
}

// Lifecycle helpers keep scheduling decisions explicit at call sites.
